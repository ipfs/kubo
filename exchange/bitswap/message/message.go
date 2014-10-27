package message

import (
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	blocks "github.com/jbenet/go-ipfs/blocks"
	pb "github.com/jbenet/go-ipfs/exchange/bitswap/message/internal/pb"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	nm "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO move message.go into the bitswap package
// TODO move bs/msg/internal/pb to bs/internal/pb and rename pb package to bitswap_pb

type BitSwapMessage interface {
	Wantlist() []u.Key
	Blocks() []blocks.Block
	AddWanted(k u.Key)
	AddBlock(b blocks.Block)
	Exportable
}

type Exportable interface {
	ToProto() *pb.Message
	ToNet(p peer.Peer) (nm.NetMessage, error)
}

type impl struct {
	wantlist map[u.Key]struct{}
	blocks   map[u.Key]blocks.Block
}

func New() BitSwapMessage {
	return &impl{
		wantlist: make(map[u.Key]struct{}),
		blocks:   make(map[u.Key]blocks.Block),
	}
}

func newMessageFromProto(pbm pb.Message) BitSwapMessage {
	m := New()
	for _, s := range pbm.GetWantlist() {
		m.AddWanted(u.Key(s))
	}
	for _, d := range pbm.GetBlocks() {
		b := blocks.NewBlock(d)
		m.AddBlock(*b)
	}
	return m
}

// TODO(brian): convert these into keys
func (m *impl) Wantlist() []u.Key {
	wl := make([]u.Key, 0)
	for k, _ := range m.wantlist {
		wl = append(wl, k)
	}
	return wl
}

// TODO(brian): convert these into blocks
func (m *impl) Blocks() []blocks.Block {
	bs := make([]blocks.Block, 0)
	for _, block := range m.blocks {
		bs = append(bs, block)
	}
	return bs
}

func (m *impl) AddWanted(k u.Key) {
	m.wantlist[k] = struct{}{}
}

func (m *impl) AddBlock(b blocks.Block) {
	m.blocks[b.Key()] = b
}

func FromNet(nmsg netmsg.NetMessage) (BitSwapMessage, error) {
	pb := new(pb.Message)
	if err := proto.Unmarshal(nmsg.Data(), pb); err != nil {
		return nil, err
	}
	m := newMessageFromProto(*pb)
	return m, nil
}

func (m *impl) ToProto() *pb.Message {
	pb := new(pb.Message)
	for _, k := range m.Wantlist() {
		pb.Wantlist = append(pb.Wantlist, string(k))
	}
	for _, b := range m.Blocks() {
		pb.Blocks = append(pb.Blocks, b.Data)
	}
	return pb
}

func (m *impl) ToNet(p peer.Peer) (nm.NetMessage, error) {
	return nm.FromObject(p, m.ToProto())
}
