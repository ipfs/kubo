package message

import (
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	blocks "github.com/jbenet/go-ipfs/blocks"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	nm "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

type BitSwapMessage interface {
	Wantlist() []u.Key
	Blocks() []blocks.Block
	AppendWanted(k u.Key)
	AppendBlock(b blocks.Block)
	Exportable
}

type Exportable interface {
	ToProto() *PBMessage
	ToNet(p *peer.Peer) (nm.NetMessage, error)
}

// message wraps a proto message for convenience
type message struct {
	wantlist []u.Key
	blocks   []blocks.Block
}

func New() *message {
	return new(message)
}

func newMessageFromProto(pbm PBMessage) (BitSwapMessage, error) {
	m := New()
	for _, s := range pbm.GetWantlist() {
		m.AppendWanted(u.Key(s))
	}
	for _, d := range pbm.GetBlocks() {
		b, err := blocks.NewBlock(d)
		if err != nil {
			return nil, err
		}
		m.AppendBlock(*b)
	}
	return m, nil
}

// TODO(brian): convert these into keys
func (m *message) Wantlist() []u.Key {
	return m.wantlist
}

// TODO(brian): convert these into blocks
func (m *message) Blocks() []blocks.Block {
	return m.blocks
}

func (m *message) AppendWanted(k u.Key) {
	m.wantlist = append(m.wantlist, k)
}

func (m *message) AppendBlock(b blocks.Block) {
	m.blocks = append(m.blocks, b)
}

func FromNet(nmsg netmsg.NetMessage) (BitSwapMessage, error) {
	pb := new(PBMessage)
	if err := proto.Unmarshal(nmsg.Data(), pb); err != nil {
		return nil, err
	}
	m, err := newMessageFromProto(*pb)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *message) ToProto() *PBMessage {
	pb := new(PBMessage)
	for _, k := range m.Wantlist() {
		pb.Wantlist = append(pb.Wantlist, string(k))
	}
	for _, b := range m.Blocks() {
		pb.Blocks = append(pb.Blocks, b.Data)
	}
	return pb
}

func (m *message) ToNet(p *peer.Peer) (nm.NetMessage, error) {
	return nm.FromObject(p, m.ToProto())
}
