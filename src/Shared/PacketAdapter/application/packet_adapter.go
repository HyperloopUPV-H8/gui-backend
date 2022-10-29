package application

import (
	excel "github.com/HyperloopUPV-H8/Backend-H8/Shared/ExcelParser/application/interfaces"
	"github.com/HyperloopUPV-H8/Backend-H8/Shared/PacketAdapter/application/interfaces"
	"github.com/HyperloopUPV-H8/Backend-H8/Shared/PacketAdapter/infra"
)

type PacketAdapter struct {
	controller interfaces.TransportController
	parser     PacketParser
}

func New(ips []string, packets []excel.Packet) PacketAdapter {
	return PacketAdapter{
		controller: infra.NewTransportController(ips),
		parser:     NewParser(packets),
	}
}

func (adapter PacketAdapter) ReadData() interfaces.PacketUpdate {
	payload := adapter.controller.ReceiveData()
	return adapter.parser.Decode(payload)
}
