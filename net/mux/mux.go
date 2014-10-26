package mux

import (
	"errors"
	"sync"

	msg "github.com/jbenet/go-ipfs/net/message"
	pb "github.com/jbenet/go-ipfs/net/mux/internal/pb"
	u "github.com/jbenet/go-ipfs/util"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

var log = u.Logger("muxer")

// ProtocolIDs used to identify each protocol.
// These should probably be defined elsewhere.
var (
	ProtocolID_Routing    = pb.ProtocolID_Routing
	ProtocolID_Exchange   = pb.ProtocolID_Exchange
	ProtocolID_Diagnostic = pb.ProtocolID_Diagnostic
)

// Protocol objects produce + consume raw data. They are added to the Muxer
// with a ProtocolID, which is added to outgoing payloads. Muxer properly
// encapsulates and decapsulates when interfacing with its Protocols. The
// Protocols do not encounter their ProtocolID.
type Protocol interface {
	GetPipe() *msg.Pipe
}

// ProtocolMap maps ProtocolIDs to Protocols.
type ProtocolMap map[pb.ProtocolID]Protocol

// Muxer is a simple multiplexor that reads + writes to Incoming and Outgoing
// channels. It multiplexes various protocols, wrapping and unwrapping data
// with a ProtocolID.
type Muxer struct {
	// Protocols are the multiplexed services.
	Protocols ProtocolMap

	bwiLock sync.Mutex
	bwIn    uint64

	bwoLock sync.Mutex
	bwOut   uint64

	*msg.Pipe
	ctxc.ContextCloser
}

// NewMuxer constructs a muxer given a protocol map.
func NewMuxer(ctx context.Context, mp ProtocolMap) *Muxer {
	m := &Muxer{
		Protocols:     mp,
		Pipe:          msg.NewPipe(10),
		ContextCloser: ctxc.NewContextCloser(ctx, nil),
	}

	m.Children().Add(1)
	go m.handleIncomingMessages()
	for pid, proto := range m.Protocols {
		m.Children().Add(1)
		go m.handleOutgoingMessages(pid, proto)
	}

	return m
}

// GetPipe implements the Protocol interface
func (m *Muxer) GetPipe() *msg.Pipe {
	return m.Pipe
}

// GetBandwidthTotals return the in/out bandwidth measured over this muxer.
func (m *Muxer) GetBandwidthTotals() (in uint64, out uint64) {
	m.bwiLock.Lock()
	in = m.bwIn
	m.bwiLock.Unlock()

	m.bwoLock.Lock()
	out = m.bwOut
	m.bwoLock.Unlock()
	return
}

// AddProtocol adds a Protocol with given ProtocolID to the Muxer.
func (m *Muxer) AddProtocol(p Protocol, pid pb.ProtocolID) error {
	if _, found := m.Protocols[pid]; found {
		return errors.New("Another protocol already using this ProtocolID")
	}

	m.Protocols[pid] = p
	return nil
}

// handleIncoming consumes the messages on the m.Incoming channel and
// routes them appropriately (to the protocols).
func (m *Muxer) handleIncomingMessages() {
	defer m.Children().Done()

	for {
		select {
		case <-m.Closing():
			return

		case msg, more := <-m.Incoming:
			if !more {
				return
			}
			m.Children().Add(1)
			go m.handleIncomingMessage(msg)
		}
	}
}

// handleIncomingMessage routes message to the appropriate protocol.
func (m *Muxer) handleIncomingMessage(m1 msg.NetMessage) {
	defer m.Children().Done()

	m.bwiLock.Lock()
	// TODO: compensate for overhead
	m.bwIn += uint64(len(m1.Data()))
	m.bwiLock.Unlock()

	data, pid, err := unwrapData(m1.Data())
	if err != nil {
		log.Errorf("muxer de-serializing error: %v", err)
		return
	}

	m2 := msg.New(m1.Peer(), data)
	proto, found := m.Protocols[pid]
	if !found {
		log.Errorf("muxer unknown protocol %v", pid)
		return
	}

	select {
	case proto.GetPipe().Incoming <- m2:
	case <-m.Closing():
		return
	}
}

// handleOutgoingMessages consumes the messages on the proto.Outgoing channel,
// wraps them and sends them out.
func (m *Muxer) handleOutgoingMessages(pid pb.ProtocolID, proto Protocol) {
	defer m.Children().Done()

	for {
		select {
		case msg, more := <-proto.GetPipe().Outgoing:
			if !more {
				return
			}
			m.Children().Add(1)
			go m.handleOutgoingMessage(pid, msg)

		case <-m.Closing():
			return
		}
	}
}

// handleOutgoingMessage wraps out a message and sends it out the
func (m *Muxer) handleOutgoingMessage(pid pb.ProtocolID, m1 msg.NetMessage) {
	defer m.Children().Done()

	data, err := wrapData(m1.Data(), pid)
	if err != nil {
		log.Errorf("muxer serializing error: %v", err)
		return
	}

	m.bwoLock.Lock()
	// TODO: compensate for overhead
	// TODO(jbenet): switch this to a goroutine to prevent sync waiting.
	m.bwOut += uint64(len(data))
	m.bwoLock.Unlock()

	m2 := msg.New(m1.Peer(), data)
	select {
	case m.GetPipe().Outgoing <- m2:
	case <-m.Closing():
		return
	}
}

func wrapData(data []byte, pid pb.ProtocolID) ([]byte, error) {
	// Marshal
	pbm := new(pb.PBProtocolMessage)
	pbm.ProtocolID = &pid
	pbm.Data = data
	b, err := proto.Marshal(pbm)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func unwrapData(data []byte) ([]byte, pb.ProtocolID, error) {
	// Unmarshal
	pbm := new(pb.PBProtocolMessage)
	err := proto.Unmarshal(data, pbm)
	if err != nil {
		return nil, 0, err
	}

	return pbm.GetData(), pbm.GetProtocolID(), nil
}
