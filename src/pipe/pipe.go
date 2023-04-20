package pipe

import (
	"errors"
	"net"
	"time"

	"github.com/HyperloopUPV-H8/Backend-H8/packet"
	"github.com/rs/zerolog"
	trace "github.com/rs/zerolog/log"
)

type Pipe struct {
	conn *net.TCPConn

	laddr *net.TCPAddr
	raddr *net.TCPAddr

	isClosed bool
	mtu      int

	output             chan<- packet.Raw
	onConnectionChange func(bool)

	trace zerolog.Logger
}

func New(laddr string, raddr string, mtu uint, outputChan chan<- packet.Raw, onConnectionChange func(bool)) (*Pipe, error) {
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

// FIXME: si las placas no cierran la conexión bien, el back peta (hacer prueba con board_conn)
func (pipe *Pipe) connect() {
	pipe.trace.Debug().Msg("connecting")
	for pipe.isClosed {
		pipe.trace.Trace().Msg("dial")
		if conn, err := net.DialTCP("tcp", pipe.laddr, pipe.raddr); err == nil {
			pipe.open(conn)
		} else {
			pipe.trace.Trace().Stack().Err(err).Msg("dial failed")
		}
	}
	pipe.trace.Info().Msg("connected")

	go pipe.listen()
}

func (pipe *Pipe) open(conn *net.TCPConn) {
	pipe.trace.Debug().Msg("open")
	pipe.conn = conn
	pipe.isClosed = false
	pipe.onConnectionChange(!pipe.isClosed)
}

func (pipe *Pipe) listen() {
	pipe.trace.Info().Msg("start listening")
	for {
		buffer := make([]byte, pipe.mtu)
		n, err := pipe.conn.Read(buffer)
		if err != nil {
			pipe.trace.Error().Stack().Err(err).Msg("")
			pipe.Close(true)
			return
		}

		if pipe.output == nil {
			pipe.trace.Debug().Msg("no output configured")
			continue
		}

		pipe.trace.Trace().Msg("new message")

		raw := pipe.getRaw(buffer[:n])

		pipe.output <- raw
	}
}

var syntheticSeqNum uint32 = 0

func (pipe *Pipe) getRaw(payload []byte) packet.Raw {
	syntheticSeqNum++
	return packet.Raw{
		Metadata: packet.NewMetaData(pipe.raddr.String(), pipe.laddr.String(), 0, syntheticSeqNum, time.Now()),
		Payload:  payload,
	}
}

func (pipe *Pipe) Write(data []byte) (int, error) {
	if pipe == nil || pipe.conn == nil {
		err := errors.New("pipe is nil")
		pipe.trace.Error().Stack().Err(err).Msg("")
		return 0, err
	}

	pipe.trace.Trace().Msg("write")
	return pipe.conn.Write(data)
}

func (pipe *Pipe) Close(reconnect bool) error {
	pipe.trace.Warn().Bool("reconnect", reconnect).Msg("close")

	err := pipe.conn.Close()
	pipe.isClosed = err == nil
	pipe.onConnectionChange(!pipe.isClosed)

	if reconnect {
		go pipe.connect()
	}
	return err
}

func (pipe *Pipe) Laddr() string {
	return pipe.laddr.String()
}

func (pipe *Pipe) Raddr() string {
	return pipe.raddr.String()
}
