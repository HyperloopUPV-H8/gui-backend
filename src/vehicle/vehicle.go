package vehicle

import (
	"fmt"
	"sync"

	"github.com/HyperloopUPV-H8/Backend-H8/common"
	"github.com/HyperloopUPV-H8/Backend-H8/message_parser"
	"github.com/HyperloopUPV-H8/Backend-H8/packet_parser"
	"github.com/HyperloopUPV-H8/Backend-H8/pipe"
	"github.com/HyperloopUPV-H8/Backend-H8/sniffer"
	"github.com/HyperloopUPV-H8/Backend-H8/unit_converter"
	"github.com/HyperloopUPV-H8/Backend-H8/vehicle/internals"
	"github.com/HyperloopUPV-H8/Backend-H8/vehicle/models"
	"github.com/rs/zerolog"
)

type Vehicle struct {
	sniffer          sniffer.Sniffer
	parser           packet_parser.PacketParser
	messageParser    message_parser.MessageParser
	displayConverter unit_converter.UnitConverter
	podConverter     unit_converter.UnitConverter
	pipes            map[string]*pipe.Pipe

	packetFactory internals.UpdateFactory

	updateChan  chan []byte
	messageChan chan []byte

	idToBoard map[uint16]string

	stats              Stats
	statsMx            *sync.Mutex
	onConnectionChange func(string, bool)

	trace zerolog.Logger
}

func (vehicle *Vehicle) Listen(updateChan chan<- models.Update, messagesChan chan<- any) {
	vehicle.trace.Info().Msg("start listening")
	go vehicle.listenData(updateChan)

	go vehicle.listenMessages(messagesChan)
}

func (vehicle *Vehicle) listenData(dataChan chan<- models.Update) {
	for raw := range vehicle.updateChan {
		rawCopy := make([]byte, len(raw))
		copy(rawCopy, raw)

		id, fields := vehicle.parser.Decode(rawCopy)
		fields = vehicle.podConverter.Revert(fields)
		fields = vehicle.displayConverter.Convert(fields)

		update := vehicle.packetFactory.NewUpdate(id, rawCopy, fields)
		vehicle.statsMx.Lock()
		vehicle.stats.recv++
		vehicle.statsMx.Unlock()

		vehicle.trace.Trace().Uint16("id", id).Msg("read data")
		dataChan <- update
	}
}

func (vehicle *Vehicle) listenMessages(messageChan chan<- any) {
	for raw := range vehicle.messageChan {
		msg, err := vehicle.messageParser.Parse(raw)
		if err != nil {
			vehicle.trace.Error().Stack().Err(err).Str("raw", fmt.Sprintf("%#v", string(raw))).Msg("parse message")
			continue
		}
		messageChan <- msg
	}
}

func (vehicle *Vehicle) SendOrder(order models.Order) error {
	vehicle.trace.Info().Uint16("id", order.ID).Msg("send order")
	pipe, ok := vehicle.pipes[vehicle.idToBoard[order.ID]]
	if !ok {
		err := fmt.Errorf("%s pipe for %d not found", vehicle.idToBoard[order.ID], order.ID)
		vehicle.trace.Error().Stack().Err(err).Msg("")
		return err
	}

	fields := order.Fields
	fields = vehicle.displayConverter.Convert(fields)
	fields = vehicle.podConverter.Revert(fields)
	raw := vehicle.parser.Encode(order.ID, fields)

	_, err := common.WriteAll(pipe, raw)

	vehicle.statsMx.Lock()
	defer vehicle.statsMx.Unlock()

	if err == nil {
		vehicle.stats.sent++
	} else {
		vehicle.trace.Error().Stack().Err(err).Msg("")
		vehicle.stats.sentFail++
	}

	return err
}
