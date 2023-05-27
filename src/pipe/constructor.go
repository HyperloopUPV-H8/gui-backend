package pipe

import (
	"net"
	"os"

	"github.com/HyperloopUPV-H8/Backend-H8/common"
	excel_models "github.com/HyperloopUPV-H8/Backend-H8/excel_adapter/models"
	"github.com/HyperloopUPV-H8/Backend-H8/packet"
	"github.com/rs/zerolog"
	trace "github.com/rs/zerolog/log"
)

func CreatePipes(global excel_models.GlobalInfo, dataChan chan<- packet.Packet, onConnectionChange func(string, bool), config Config, trace zerolog.Logger) map[string]*Pipe {
	laddr := common.AddrWithPort(global.BackendIP, global.ProtocolToPort[config.TcpClientTag])
	pipes := make(map[string]*Pipe)

	if os.Getenv("APP_ENV") == "control" {
		raddr := common.AddrWithPort(global.BoardToIP["vcu"], global.ProtocolToPort[config.TcpServerTag])
		pipe, err := newPipe(laddr, raddr, config.Mtu, dataChan, getOnConnectionChange("vcu", onConnectionChange))
		if err != nil {
			trace.Fatal().Stack().Err(err).Msg("error creating pipe")
		}
		pipes["vcu"] = pipe
		return pipes
	}

	for board, ip := range global.BoardToIP {
		raddr := common.AddrWithPort(ip, global.ProtocolToPort[config.TcpServerTag])
		pipe, err := newPipe(laddr, raddr, config.Mtu, dataChan, getOnConnectionChange(board, onConnectionChange))
		if err != nil {
			trace.Fatal().Stack().Err(err).Msg("error creating pipe")
		}

		pipes[board] = pipe
	}
	return pipes
}

func newPipe(laddr string, raddr string, mtu uint, outputChan chan<- packet.Packet, onConnectionChange func(bool)) (*Pipe, error) {
	trace.Info().Str("laddr", laddr).Str("raddr", raddr).Msg("new pipe")
	localAddr, err := net.ResolveTCPAddr("tcp", laddr)
	if err != nil {
		trace.Error().Str("laddr", laddr).Stack().Err(err).Msg("")
		return nil, err
	}

	remoteAddr, err := net.ResolveTCPAddr("tcp", raddr)
	if err != nil {
		trace.Error().Str("raddr", raddr).Stack().Err(err).Msg("")
		return nil, err
	}

	pipe := &Pipe{
		laddr:  localAddr,
		raddr:  remoteAddr,
		output: outputChan,

		isClosed: true,
		mtu:      int(mtu),

		onConnectionChange: onConnectionChange,

		trace: trace.With().Str("component", "pipe").IPAddr("addr", remoteAddr.IP).Logger(),
	}

	go pipe.connect()

	return pipe, nil
}

func getOnConnectionChange(board string, onConnectionChange func(string, bool)) func(bool) {
	return func(state bool) {
		onConnectionChange(board, state)
	}
}
