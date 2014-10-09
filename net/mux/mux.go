package mux

import (
	"errors"
	"sync"

	msg "github.com/jbenet/go-ipfs/net/message"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

var log = u.Logger("muxer")

// Protocol objects produce + consume raw data. They are added to the Muxer
// with a ProtocolID, which is added to outgoing payloads. Muxer properly
// encapsulates and decapsulates when interfacing with its Protocols. The
// Protocols do not encounter their ProtocolID.
type Protocol interface {
	GetPipe() *msg.Pipe
}

// ProtocolMap maps ProtocolIDs to Protocols.
type ProtocolMap map[ProtocolID]Protocol

// Muxer is a simple multiplexor that reads + writes to Incoming and Outgoing
// channels. It multiplexes various protocols, wrapping and unwrapping data
// with a ProtocolID.
type Muxer struct {
	// Protocols are the multiplexed services.
	Protocols ProtocolMap

	// cancel is the function to stop the Muxer
	cancel context.CancelFunc
	ctx    context.Context
	wg     sync.WaitGroup

	*msg.Pipe
}

// NewMuxer constructs a muxer given a protocol map.
func NewMuxer(mp ProtocolMap) *Muxer {
	return &Muxer{
		Protocols: mp,
		Pipe:      msg.NewPipe(10),
	}
}

// GetPipe implements the Protocol interface
func (m *Muxer) GetPipe() *msg.Pipe {
	return m.Pipe
}

// Start kicks off the Muxer goroutines.
func (m *Muxer) Start(ctx context.Context) error {
	if m == nil {
		panic("nix muxer")
	}

	if m.cancel != nil {
		return errors.New("Muxer already started.")
	}

	// make a cancellable context.
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.wg = sync.WaitGroup{}

	m.wg.Add(1)
	go m.handleIncomingMessages()
	for pid, proto := range m.Protocols {
		m.wg.Add(1)
		go m.handleOutgoingMessages(pid, proto)
	}

	return nil
}

// Stop stops muxer activity.
func (m *Muxer) Stop() {
	if m.cancel == nil {
		panic("muxer stopped twice.")
	}
	// issue cancel, and wipe func.
	m.cancel()
	m.cancel = context.CancelFunc(nil)

	// wait for everything to wind down.
	m.wg.Wait()
}

// AddProtocol adds a Protocol with given ProtocolID to the Muxer.
func (m *Muxer) AddProtocol(p Protocol, pid ProtocolID) error {
	if _, found := m.Protocols[pid]; found {
		return errors.New("Another protocol already using this ProtocolID")
	}

	m.Protocols[pid] = p
	return nil
}

// handleIncoming consumes the messages on the m.Incoming channel and
// routes them appropriately (to the protocols).
func (m *Muxer) handleIncomingMessages() {
	defer m.wg.Done()

	for {
		if m == nil {
			panic("nil muxer")
		}

		select {
		case msg, more := <-m.Incoming:
			if !more {
				return
			}
			go m.handleIncomingMessage(msg)

		case <-m.ctx.Done():
			return
		}
	}
}

// handleIncomingMessage routes message to the appropriate protocol.
func (m *Muxer) handleIncomingMessage(m1 msg.NetMessage) {

	data, pid, err := unwrapData(m1.Data())
	if err != nil {
		log.Error("muxer de-serializing error: %v", err)
		return
	}

	m2 := msg.New(m1.Peer(), data)
	proto, found := m.Protocols[pid]
	if !found {
		log.Error("muxer unknown protocol %v", pid)
		return
	}

	select {
	case proto.GetPipe().Incoming <- m2:
	case <-m.ctx.Done():
		log.Error("%s", m.ctx.Err())
		return
	}
}

// handleOutgoingMessages consumes the messages on the proto.Outgoing channel,
// wraps them and sends them out.
func (m *Muxer) handleOutgoingMessages(pid ProtocolID, proto Protocol) {
	defer m.wg.Done()

	for {
		select {
		case msg, more := <-proto.GetPipe().Outgoing:
			if !more {
				return
			}
			go m.handleOutgoingMessage(pid, msg)

		case <-m.ctx.Done():
			return
		}
	}
}

// handleOutgoingMessage wraps out a message and sends it out the
func (m *Muxer) handleOutgoingMessage(pid ProtocolID, m1 msg.NetMessage) {
	data, err := wrapData(m1.Data(), pid)
	if err != nil {
		log.Error("muxer serializing error: %v", err)
		return
	}

	m2 := msg.New(m1.Peer(), data)
	select {
	case m.GetPipe().Outgoing <- m2:
	case <-m.ctx.Done():
		return
	}
}

func wrapData(data []byte, pid ProtocolID) ([]byte, error) {
	// Marshal
	pbm := new(PBProtocolMessage)
	pbm.ProtocolID = &pid
	pbm.Data = data
	b, err := proto.Marshal(pbm)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func unwrapData(data []byte) ([]byte, ProtocolID, error) {
	// Unmarshal
	pbm := new(PBProtocolMessage)
	err := proto.Unmarshal(data, pbm)
	if err != nil {
		return nil, 0, err
	}

	return pbm.GetData(), pbm.GetProtocolID(), nil
}
