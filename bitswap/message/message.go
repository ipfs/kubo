package message

import (
	"errors"

	netmsg "github.com/jbenet/go-ipfs/net/message"

	blocks "github.com/jbenet/go-ipfs/blocks"
	nm "github.com/jbenet/go-ipfs/net/message"
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

func (m *message) ToProto() *PBMessage {
	cp := m.pb
	return &cp
}

func (m *message) ToNet(p *peer.Peer) (nm.NetMessage, error) {
	return nm.FromObject(p, m.ToProto())
}
