package message

import (
	peer "github.com/jbenet/go-ipfs/peer"

	proto "code.google.com/p/goprotobuf/proto"
)

// Message represents a packet of information sent to or received from a
// particular Peer.
type Message struct {
	// To or from, depending on direction.
	Peer *peer.Peer

	// Opaque data
	Data []byte
}

// FromObject creates a message from a protobuf-marshallable message.
func FromObject(p *peer.Peer, data proto.Message) (*Message, error) {
	bytes, err := proto.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Message{
		Peer: p,
		Data: bytes,
	}, nil
}

// Pipe objects represent a bi-directional message channel.
type Pipe struct {
	Incoming chan *Message
	Outgoing chan *Message
}

// NewPipe constructs a pipe with channels of a given buffer size.
func NewPipe(bufsize int) *Pipe {
	return &Pipe{
		Incoming: make(chan *Message, bufsize),
		Outgoing: make(chan *Message, bufsize),
	}
}
