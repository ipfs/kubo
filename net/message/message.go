package message

import (
	peer "github.com/jbenet/go-ipfs/peer"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

type NetMessage interface {
	Peer() *peer.Peer
	Data() []byte
}

func New(p *peer.Peer, data []byte) NetMessage {
	return &message{peer: p, data: data}
}

// message represents a packet of information sent to or received from a
// particular Peer.
type message struct {
	// To or from, depending on direction.
	peer *peer.Peer

	// Opaque data
	data []byte
}

func (m *message) Peer() *peer.Peer {
	return m.peer
}

func (m *message) Data() []byte {
	return m.data
}

// FromObject creates a message from a protobuf-marshallable message.
func FromObject(p *peer.Peer, data proto.Message) (NetMessage, error) {
	bytes, err := proto.Marshal(data)
	if err != nil {
		return nil, err
	}
	return New(p, bytes), nil
}

// Pipe objects represent a bi-directional message channel.
type Pipe struct {
	Incoming chan NetMessage
	Outgoing chan NetMessage
}

// NewPipe constructs a pipe with channels of a given buffer size.
func NewPipe(bufsize int) *Pipe {
	return &Pipe{
		Incoming: make(chan NetMessage, bufsize),
		Outgoing: make(chan NetMessage, bufsize),
	}
}
