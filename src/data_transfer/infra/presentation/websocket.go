package presentation

import (
	"github.com/HyperloopUPV-H8/Backend-H8/Shared/server/infra/interfaces"
	"github.com/HyperloopUPV-H8/Backend-H8/data_transfer/application"
)

type id = uint16

func DataRoutine(socket interfaces.WebSocket, data <-chan application.PacketJSON) {
	go func() {
		var err error
		for err == nil {
			err = socket.WriteJSON([]application.PacketJSON{<-data})
		}
	}()
}
