package message

import (
	"errors"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	netmsg "github.com/jbenet/go-ipfs/net/message"

	blocks "github.com/jbenet/go-ipfs/blocks"
	nm "github.com/jbenet/go-ipfs/net/message"
	swarm "github.com/jbenet/go-ipfs/net/swarm"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

type BitSwapMessage interface {
	Wantlist() []u.Key
	Blocks() []blocks.Block
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

func New() *message {
	return new(message)
}

// TODO(brian): convert these into keys
func (m *message) Wantlist() []u.Key {
	wl := make([]u.Key, len(m.pb.Wantlist))
	for _, str := range m.pb.Wantlist {
		wl = append(wl, u.Key(str))
	}
	return wl
}

// TODO(brian): convert these into blocks
func (m *message) Blocks() []blocks.Block {
	bs := make([]blocks.Block, len(m.pb.Blocks))
	for _, data := range m.pb.Blocks {
		b, err := blocks.NewBlock(data)
		if err != nil {
			continue
		}
		bs = append(bs, *b)
	}
	return bs
}

func (m *message) AppendWanted(k u.Key) {
	m.pb.Wantlist = append(m.pb.Wantlist, string(k))
}

func (m *message) AppendBlock(b *blocks.Block) {
	m.pb.Blocks = append(m.pb.Blocks, b.Data)
}

func FromNet(nmsg netmsg.NetMessage) (BitSwapMessage, error) {
	return nil, errors.New("TODO implement")
}

func FromSwarm(sms swarm.Message) (BitSwapMessage, error) {
	var protoMsg PBMessage
	err := proto.Unmarshal(sms.Data, &protoMsg)
	if err != nil {
		return nil, err
	}
	return newMessageFromProto(protoMsg), nil
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
