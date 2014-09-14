package bitswap

import (
	blocks "github.com/jbenet/go-ipfs/blocks"
	swarm "github.com/jbenet/go-ipfs/net/swarm"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// message wraps a proto message for convenience
type message struct {
	pb PBMessage
}

func newMessageFromProto(pb PBMessage) *message {
	return &message{pb: pb}
}

func newMessage() *message {
	return new(message)
}

func (m *message) AppendWanted(k u.Key) {
	m.pb.Wantlist = append(m.pb.Wantlist, string(k))
}

func (m *message) AppendBlock(b *blocks.Block) {
	m.pb.Blocks = append(m.pb.Blocks, b.Data)
}

func (m *message) ToProto() *PBMessage {
	cp := m.pb
	return &cp
}

func (m *message) ToSwarm(p *peer.Peer) *swarm.Message {
	return swarm.NewMessage(p, m.ToProto())
}
