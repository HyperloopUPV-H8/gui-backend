package main

import (
	"flag"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/HyperloopUPV-H8/Backend-H8/board"
	"github.com/HyperloopUPV-H8/Backend-H8/connection_transfer"
	"github.com/HyperloopUPV-H8/Backend-H8/data_transfer"
	"github.com/HyperloopUPV-H8/Backend-H8/excel_adapter"
	"github.com/HyperloopUPV-H8/Backend-H8/log_handle"
	"github.com/HyperloopUPV-H8/Backend-H8/message_transfer"
	message_transfer_models "github.com/HyperloopUPV-H8/Backend-H8/message_transfer/models"
	"github.com/HyperloopUPV-H8/Backend-H8/order_transfer"
	"github.com/HyperloopUPV-H8/Backend-H8/server"
	"github.com/HyperloopUPV-H8/Backend-H8/vehicle"
	vehicle_models "github.com/HyperloopUPV-H8/Backend-H8/vehicle/models"
	"github.com/HyperloopUPV-H8/Backend-H8/websocket_broker"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	trace "github.com/rs/zerolog/log"
)

var traceLevel = flag.String("trace", "info", "set the trace level (\"fatal\", \"error\", \"warn\", \"info\", \"debug\", \"trace\")")
var traceLevelMap = map[string]zerolog.Level{
	"fatal": zerolog.FatalLevel,
	"error": zerolog.ErrorLevel,
	"warn":  zerolog.WarnLevel,
	"info":  zerolog.InfoLevel,
	"debug": zerolog.DebugLevel,
	"trace": zerolog.TraceLevel,
}

var traceFile = flag.String("log", "trace.json", "set the trace log file")

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	godotenv.Load(".env")

	flag.Parse()
	traceFile := initTrace()
	defer traceFile.Close()

	document := excel_adapter.FetchDocument(os.Getenv("EXCEL_ID"), os.Getenv("EXCEL_PATH"), os.Getenv("EXCEL_NAME"))

	vehicleBuilder := vehicle.NewBuilder()
	podData := vehicle_models.NewPodData()
	orderData := vehicle_models.NewOrderData()

	excel_adapter.Update(document, vehicleBuilder, podData, orderData)

	vehicle := vehicleBuilder.Build()

	vehicleOutput := make(chan vehicle_models.Update)
	go vehicle.Listen(vehicleOutput)

	boardMux := board.NewMux(board.WithInput(vehicleOutput), board.WithOutput(vehicle.SendOrder))

	updateChan := make(chan vehicle_models.Update)
	go boardMux.Listen(updateChan)

	// Communication with front-end
	websocketBroker := websocket_broker.Get()

	connectionTransfer := connection_transfer.Get()
	dataTransfer := data_transfer.Get()
	logger := log_handle.Get()
	messageTransfer := message_transfer.Get()
	orderTransfer, orderChannel := order_transfer.Get()

	websocketBroker.RegisterHandle(connectionTransfer, "connection/get")
	websocketBroker.RegisterHandle(dataTransfer)
	websocketBroker.RegisterHandle(logger, "logger/enable")
	websocketBroker.RegisterHandle(messageTransfer)
	websocketBroker.RegisterHandle(orderTransfer, "order/send")

	vehicle.OnConnectionChange(connectionTransfer.Update)

	idToType := getIdToType(podData)
	go func() {
		for update := range updateChan {
			logger.Update(update)
			if idToType[update.ID] == "data" {
				dataTransfer.Update(update)
			} else if msg, err := message_transfer_models.MessageFromUpdate(update); err == nil {
				messageTransfer.SendMessage(msg)
			}
		}
	}()

	go func() {
		for order := range orderChannel {
			if err := boardMux.Request(order); err != nil {
				trace.Error().Err(err).Msg("")
			}
		}
	}()

	httpServer := server.New(mux.NewRouter())

	httpServer.ServeData("/backend/"+os.Getenv("POD_DATA_ENDPOINT"), podData)
	httpServer.ServeData("/backend/"+os.Getenv("ORDER_DATA_ENDPOINT"), orderData)

	httpServer.HandleFunc("/backend", websocketBroker.HandleConn)

	path, _ := os.Getwd()
	httpServer.FileServer("/", filepath.Join(path, "static"))

	go httpServer.ListenAndServe(os.Getenv("SERVER_ADDR"))

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

loop:
	for {
		select {
		case <-time.After(time.Second * 10):
			trace.Trace().Any("stats", vehicle.Stats()).Msg("stats")
		case <-interrupt:
			break loop
		}
	}
}

func getIdToType(podData *vehicle_models.PodData) map[uint16]string {
	idToType := make(map[uint16]string)
	for _, brd := range podData.Boards {
		for _, pkt := range brd.Packets {
			idToType[pkt.ID] = "data"
		measurements_loop:
			for msr := range pkt.Measurements {
				if msr == "warning" {
					idToType[pkt.ID] = "warning"
					break measurements_loop
				} else if msr == "fault" {
					idToType[pkt.ID] = "fault"
					break measurements_loop
				}
			}
		}
	}
	return idToType
}
