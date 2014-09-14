package mux

import (
	"errors"

	msg "github.com/jbenet/go-ipfs/net/message"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

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

	*msg.Pipe
}

// GetPipe implements the Protocol interface
func (m *Muxer) GetPipe() *msg.Pipe {
	return m.Pipe
}

// Start kicks off the Muxer goroutines.
func (m *Muxer) Start(ctx context.Context) error {
	if m.cancel != nil {
		return errors.New("Muxer already started.")
	}

	// make a cancellable context.
	ctx, m.cancel = context.WithCancel(ctx)

	go m.handleIncomingMessages(ctx)
	for pid, proto := range m.Protocols {
		go m.handleOutgoingMessages(ctx, pid, proto)
	}

	return nil
}

// Stop stops muxer activity.
func (m *Muxer) Stop() {
	m.cancel()
	m.cancel = context.CancelFunc(nil)
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
func (m *Muxer) handleIncomingMessages(ctx context.Context) {
	for {
		select {
		case msg := <-m.Incoming:
			go m.handleIncomingMessage(ctx, msg)

		case <-ctx.Done():
			return
		}
	}
}

// handleIncomingMessage routes message to the appropriate protocol.
func (m *Muxer) handleIncomingMessage(ctx context.Context, m1 msg.NetMessage) {

	data, pid, err := unwrapData(m1.Data())
	if err != nil {
		u.PErr("muxer de-serializing error: %v\n", err)
		return
	}

	m2 := msg.New(m1.Peer(), data)
	proto, found := m.Protocols[pid]
	if !found {
		u.PErr("muxer unknown protocol %v\n", pid)
		return
	}

	select {
	case proto.GetPipe().Incoming <- m2:
	case <-ctx.Done():
		u.PErr("%v\n", ctx.Err())
		return
	}
}

// handleOutgoingMessages consumes the messages on the proto.Outgoing channel,
// wraps them and sends them out.
func (m *Muxer) handleOutgoingMessages(ctx context.Context, pid ProtocolID, proto Protocol) {
	for {
		select {
		case msg := <-proto.GetPipe().Outgoing:
			go m.handleOutgoingMessage(ctx, pid, msg)

		case <-ctx.Done():
			return
		}
	}
}

// handleOutgoingMessage wraps out a message and sends it out the
func (m *Muxer) handleOutgoingMessage(ctx context.Context, pid ProtocolID, m1 msg.NetMessage) {
	data, err := wrapData(m1.Data(), pid)
	if err != nil {
		u.PErr("muxer serializing error: %v\n", err)
		return
	}

	m2 := msg.New(m1.Peer(), data)
	select {
	case m.GetPipe().Outgoing <- m2:
	case <-ctx.Done():
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
