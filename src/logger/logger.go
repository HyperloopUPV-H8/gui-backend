package logger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/HyperloopUPV-H8/Backend-H8/common"
	"github.com/HyperloopUPV-H8/Backend-H8/logger/models"
	vehicle_models "github.com/HyperloopUPV-H8/Backend-H8/vehicle/models"
	"github.com/rs/zerolog"
	trace "github.com/rs/zerolog/log"
)

const (
	LOG_HANDLE_HANDLER_NAME = "logHandle"
	UPDATE_CHAN_BUF         = 100
)

type Logger struct {
	files    map[string]*os.File
	buffer   map[string][]models.Value
	autosave *time.Ticker

	done         chan struct{}
	innerUpdates chan vehicle_models.Update

	Updates chan vehicle_models.Update
	Enable  chan EnableMsg

	isRunning   bool
	isRunningMx sync.Mutex

	client string

	topics struct {
		Enable string
		State  string
	}
	sendMessage func(topic string, payload any, target ...string) error

	path          string
	dumpThreshold uint

	trace zerolog.Logger
}
type LoggerConfig struct {
	DumpSize      uint   `toml:"dump_size"`
	RowSize       uint   `toml:"row_size"`
	AutosaveDelay string `toml:"autosave_delay"`
	Path          string
	Topics        LoggerTopics
}

type LoggerTopics struct {
	Enable string
	State  string
}

type EnableMsg struct {
	Client string
	Enable bool
}

func New(config LoggerConfig) Logger {
	trace.Info().Msg("new log handle")

	autosaveDelay, err := time.ParseDuration(config.AutosaveDelay)
	if err != nil {
		trace.Fatal().Stack().Err(err).Str("LOGGER_AUTOSAVE_DELAY", config.AutosaveDelay).Msg("")
	}

	return Logger{
		files:    make(map[string]*os.File),
		buffer:   make(map[string][]models.Value),
		autosave: time.NewTicker(autosaveDelay),

		done: make(chan struct{}),

		Updates:      make(chan vehicle_models.Update, UPDATE_CHAN_BUF),
		innerUpdates: make(chan vehicle_models.Update, UPDATE_CHAN_BUF),

		Enable: make(chan EnableMsg),

		isRunning:   false,
		isRunningMx: sync.Mutex{},
		client:      "",

		topics:      LoggerTopics{Enable: config.Topics.Enable, State: config.Topics.State},
		sendMessage: defaultSendMessage,

		path:          config.Path,
		dumpThreshold: config.DumpSize / config.RowSize,

		trace: trace.With().Str("component", LOG_HANDLE_HANDLER_NAME).Logger(),
	}
}

func (logger *Logger) UpdateMessage(topic string, payload json.RawMessage, source string) {
	logger.trace.Debug().Str("topic", topic).Str("source", source).Msg("update message")
	switch topic {
	case logger.topics.Enable:
		var enable bool
		if err := json.Unmarshal(payload, &enable); err != nil {
			logger.trace.Error().Stack().Err(err).Msg("")
			return
		}

		logger.Enable <- EnableMsg{
			Client: source,
			Enable: enable,
		}
	}
}

// logger.isRunningMx must be locked
func (logger *Logger) notifyState() {
	logger.trace.Trace().Bool("running", logger.isRunning).Msg("notify state")
	if err := logger.sendMessage(logger.topics.State, logger.isRunning); err != nil {
		logger.trace.Error().Stack().Err(err).Msg("")
	}
}

func (logger *Logger) Listen() {
	for {
		select {
		case enableMsg := <-logger.Enable:
			if logger.client != "" && enableMsg.Client != logger.client {
				logger.trace.Warn().Str("source", enableMsg.Client).Msg("tried to change running log session")
				return
			}

			logger.isRunningMx.Lock()

			if enableMsg.Enable && !logger.isRunning {
				logger.start(enableMsg.Client)
			} else if !enableMsg.Enable && logger.isRunning {
				logger.stop()
			}

			logger.isRunningMx.Unlock()

		case update := <-logger.Updates:
			common.TrySend(logger.innerUpdates, update)
		}
	}
}

func (logger *Logger) run() {
	logger.trace.Info().Msg("run")
	for {
		select {
		case update := <-logger.innerUpdates:
			for name, value := range update.Fields {
				logger.buffer[name] = append(logger.buffer[name], models.Value{
					Value:     value,
					Timestamp: time.Now(),
				})
			}

			logger.checkDump()
		case <-logger.autosave.C:
			logger.flush()
		case <-logger.done:
			return
		}
	}
}

func (logger *Logger) checkDump() {
	logger.trace.Trace().Msg("check dump")
	for _, buf := range logger.buffer {
		if len(buf) > int(logger.dumpThreshold) {
			logger.flush()
			break
		}
	}
}

func (logger *Logger) start(client string) {
	logger.trace.Debug().Msg("start logger")
	logger.buffer = make(map[string][]models.Value)
	logger.client = client
	logger.isRunning = true
	logger.notifyState()
	go logger.run()
}

func (logger *Logger) stop() {
	logger.trace.Debug().Msg("stop logger")
	logger.client = ""
	logger.isRunning = false
	logger.notifyState()
	logger.done <- struct{}{}
	logger.flush()
	logger.Close()
}

func (logger *Logger) flush() {
	logger.trace.Info().Msg("flush")
	for value, buffer := range logger.buffer {
		logger.writeCSV(value, buffer)
	}
	logger.buffer = make(map[string][]models.Value)
}

func (logger *Logger) writeCSV(valueName string, buffer []models.Value) {
	file := logger.getFile(valueName)
	data := ""
	for _, value := range buffer {
		logger.trace.Trace().Str("name", valueName).Any("value", value).Msg("write value")
		data += fmt.Sprintf("%d,\"%v\"\n", value.Timestamp.Nanosecond(), value.Value)
	}

	_, err := file.WriteString(data)
	if err != nil {
		logger.trace.Fatal().Stack().Err(err).Msg("")
		return
	}
}

func (logger *Logger) getFile(valueName string) *os.File {
	if _, ok := logger.files[valueName]; !ok {
		logger.files[valueName] = logger.createFile(valueName)
	}
	logger.trace.Trace().Str("name", valueName).Msg("get file")
	return logger.files[valueName]
}

func (logger *Logger) createFile(valueName string) *os.File {
	basePath := logger.path
	err := os.MkdirAll(filepath.Join(basePath, valueName), os.ModeDir)
	if err != nil {
		logger.trace.Fatal().Stack().Err(err).Msg("")
		return nil
	}

	path := filepath.Join(basePath, valueName, strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v.csv", time.Now()), " ", "_"), ":", "-"))
	file, err := os.Create(path)
	if err != nil {
		logger.trace.Fatal().Stack().Err(err).Msg("")
		return nil
	}

	logger.trace.Debug().Str("name", valueName).Str("path", path).Msg("create file")
	return file
}

func (logger *Logger) Close() error {
	logger.trace.Info().Msg("close")

	var err error = nil
	for _, file := range logger.files {
		if fileErr := file.Close(); err != nil {
			logger.trace.Error().Stack().Err(fileErr).Msg("")
			err = fileErr
		}
	}
	logger.files = make(map[string]*os.File, len(logger.files))
	return err
}

func (logger *Logger) SetSendMessage(sendMessage func(topic string, payload any, target ...string) error) {
	logger.trace.Debug().Msg("set message")
	logger.sendMessage = sendMessage
}

func (logger *Logger) HandlerName() string {
	return LOG_HANDLE_HANDLER_NAME
}

func defaultSendMessage(string, any, ...string) error {
	return errors.New("logger must be registered before using")
}
