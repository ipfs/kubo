package bitswap

import (
	blocks "github.com/jbenet/go-ipfs/blocks"
	nm "github.com/jbenet/go-ipfs/net/message"
	swarm "github.com/jbenet/go-ipfs/net/swarm"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

type BitSwapMessage interface {
	AppendWanted(k u.Key)
	AppendBlock(b *blocks.Block)
	Exportable
}

type Exportable interface {
	ToProto() *PBMessage
	ToSwarm(p *peer.Peer) *swarm.Message
	ToNet(p *peer.Peer) (nm.NetMessage, error)
}

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

func (m *message) ToNet(p *peer.Peer) (nm.NetMessage, error) {
	return nm.FromObject(p, m.ToProto())
}
