package message

import (
	"errors"

	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	router "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-router"
)

// ErrInvalidPayload is an error used in the router.HandlePacket implementations
var ErrInvalidPayload = errors.New("invalid packet: non-[]byte payload")

// Packet is used inside the network package to represent a message
// flowing across the subsystems (Conn, Swarm, Mux, Service).
// implements router.Packet
type Packet struct {
	Src     router.Address  // peer.ID or service string
	Dst     router.Address  // peer.ID or service string
	Data    []byte          // raw data
	Context context.Context // context of the Packet.
}

func (p *Packet) Destination() router.Address {
	return p.Dst
}

func (p *Packet) Payload() interface{} {
	return p.Data
}

func (p *Packet) Response(data []byte) Packet {
	return Packet{
		Src:     p.Dst,
		Dst:     p.Src,
		Data:    data,
		Context: p.Context,
	}
}

// NetMessage is the interface for the message
type NetMessage interface {
	Peer() peer.Peer
	Data() []byte
	Loggable() map[string]interface{}
}

// New is the interface for constructing a new message.
func New(p peer.Peer, data []byte) NetMessage {
	return &message{peer: p, data: data}
}

// message represents a packet of information sent to or received from a
// particular Peer.
type message struct {
	// To or from, depending on direction.
	peer peer.Peer

	// Opaque data
	data []byte
}

func (m *message) Peer() peer.Peer {
	return m.peer
}

func (m *message) Data() []byte {
	return m.data
}

func (m *message) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"netMessage": map[string]interface{}{
			"recipient": m.Peer().Loggable(),
			// TODO sizeBytes? bytes? lenBytes?
			"size": len(m.Data()),
		},
	}
}

// FromObject creates a message from a protobuf-marshallable message.
func FromObject(p peer.Peer, data proto.Message) (NetMessage, error) {
	bytes, err := proto.Marshal(data)
	if err != nil {
		return nil, err
	}
	return New(p, bytes), nil
}
