package message

import (
	"fmt"
	"io"

	pb "github.com/ipfs/go-ipfs/exchange/bitswap/message/pb"
	wantlist "github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	blocks "gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"

	inet "gx/ipfs/QmXoz9o2PT3tEzf7hicegwex5UgVP54n3k82K7jrWFyN86/go-libp2p-net"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// TODO move message.go into the bitswap package
// TODO move bs/msg/internal/pb to bs/internal/pb and rename pb package to bitswap_pb

type BitSwapMessage interface {
	// Wantlist returns a slice of unique keys that represent data wanted by
	// the sender.
	Wantlist() []Entry

	// Blocks returns a slice of unique blocks
	Blocks() []blocks.Block

	// AddEntry adds an entry to the Wantlist.
	AddEntry(key *cid.Cid, priority int)

	Cancel(key *cid.Cid)

	Empty() bool

	// A full wantlist is an authoritative copy, a 'non-full' wantlist is a patch-set
	Full() bool

	AddBlock(blocks.Block)
	Exportable

	Loggable() map[string]interface{}
}

type Exportable interface {
	ToProtoV0() *pb.Message
	ToProtoV1() *pb.Message
	ToNetV0(w io.Writer) error
	ToNetV1(w io.Writer) error
}

type impl struct {
	full     bool
	wantlist map[string]*Entry
	blocks   map[string]blocks.Block
}

func New(full bool) BitSwapMessage {
	return newMsg(full)
}

func newMsg(full bool) *impl {
	return &impl{
		blocks:   make(map[string]blocks.Block),
		wantlist: make(map[string]*Entry),
		full:     full,
	}
}

type Entry struct {
	*wantlist.Entry
	Cancel bool
}

func newMessageFromProto(pbm pb.Message) (BitSwapMessage, error) {
	m := newMsg(pbm.GetWantlist().GetFull())
	for _, e := range pbm.GetWantlist().GetEntries() {
		c, err := cid.Cast([]byte(e.GetBlock()))
		if err != nil {
			return nil, fmt.Errorf("incorrectly formatted cid in wantlist: %s", err)
		}
		m.addEntry(c, int(e.GetPriority()), e.GetCancel())
	}

	// deprecated
	for _, d := range pbm.GetBlocks() {
		// CIDv0, sha256, protobuf only
		b := blocks.NewBlock(d)
		m.AddBlock(b)
	}
	//

	for _, b := range pbm.GetPayload() {
		pref, err := cid.PrefixFromBytes(b.GetPrefix())
		if err != nil {
			return nil, err
		}

		c, err := pref.Sum(b.GetData())
		if err != nil {
			return nil, err
		}

		blk, err := blocks.NewBlockWithCid(b.GetData(), c)
		if err != nil {
			return nil, err
		}

		m.AddBlock(blk)
	}

	return m, nil
}

func (m *impl) Full() bool {
	return m.full
}

func (m *impl) Empty() bool {
	return len(m.blocks) == 0 && len(m.wantlist) == 0
}

func (m *impl) Wantlist() []Entry {
	out := make([]Entry, 0, len(m.wantlist))
	for _, e := range m.wantlist {
		out = append(out, *e)
	}
	return out
}

func (m *impl) Blocks() []blocks.Block {
	bs := make([]blocks.Block, 0, len(m.blocks))
	for _, block := range m.blocks {
		bs = append(bs, block)
	}
	return bs
}

func (m *impl) Cancel(k *cid.Cid) {
	delete(m.wantlist, k.KeyString())
	m.addEntry(k, 0, true)
}

func (m *impl) AddEntry(k *cid.Cid, priority int) {
	m.addEntry(k, priority, false)
}

func (m *impl) addEntry(c *cid.Cid, priority int, cancel bool) {
	k := c.KeyString()
	e, exists := m.wantlist[k]
	if exists {
		e.Priority = priority
		e.Cancel = cancel
	} else {
		m.wantlist[k] = &Entry{
			Entry: &wantlist.Entry{
				Cid:      c,
				Priority: priority,
			},
			Cancel: cancel,
		}
	}
}

func (m *impl) AddBlock(b blocks.Block) {
	m.blocks[b.Cid().KeyString()] = b
}

func FromNet(r io.Reader) (BitSwapMessage, error) {
	pbr := ggio.NewDelimitedReader(r, inet.MessageSizeMax)
	return FromPBReader(pbr)
}

func FromPBReader(pbr ggio.Reader) (BitSwapMessage, error) {
	pb := new(pb.Message)
	if err := pbr.ReadMsg(pb); err != nil {
		return nil, err
	}

	return newMessageFromProto(*pb)
}

func (m *impl) ToProtoV0() *pb.Message {
	pbm := new(pb.Message)
	pbm.Wantlist = new(pb.Message_Wantlist)
	pbm.Wantlist.Entries = make([]*pb.Message_Wantlist_Entry, 0, len(m.wantlist))
	for _, e := range m.wantlist {
		pbm.Wantlist.Entries = append(pbm.Wantlist.Entries, &pb.Message_Wantlist_Entry{
			Block:    proto.String(e.Cid.KeyString()),
			Priority: proto.Int32(int32(e.Priority)),
			Cancel:   proto.Bool(e.Cancel),
		})
	}
	pbm.Wantlist.Full = proto.Bool(m.full)

	blocks := m.Blocks()
	pbm.Blocks = make([][]byte, 0, len(blocks))
	for _, b := range blocks {
		pbm.Blocks = append(pbm.Blocks, b.RawData())
	}
	return pbm
}

func (m *impl) ToProtoV1() *pb.Message {
	pbm := new(pb.Message)
	pbm.Wantlist = new(pb.Message_Wantlist)
	pbm.Wantlist.Entries = make([]*pb.Message_Wantlist_Entry, 0, len(m.wantlist))
	for _, e := range m.wantlist {
		pbm.Wantlist.Entries = append(pbm.Wantlist.Entries, &pb.Message_Wantlist_Entry{
			Block:    proto.String(e.Cid.KeyString()),
			Priority: proto.Int32(int32(e.Priority)),
			Cancel:   proto.Bool(e.Cancel),
		})
	}
	pbm.Wantlist.Full = proto.Bool(m.full)

	blocks := m.Blocks()
	pbm.Payload = make([]*pb.Message_Block, 0, len(blocks))
	for _, b := range blocks {
		blk := &pb.Message_Block{
			Data:   b.RawData(),
			Prefix: b.Cid().Prefix().Bytes(),
		}
		pbm.Payload = append(pbm.Payload, blk)
	}
	return pbm
}

func (m *impl) ToNetV0(w io.Writer) error {
	pbw := ggio.NewDelimitedWriter(w)

	return pbw.WriteMsg(m.ToProtoV0())
}

func (m *impl) ToNetV1(w io.Writer) error {
	pbw := ggio.NewDelimitedWriter(w)

	return pbw.WriteMsg(m.ToProtoV1())
}

func (m *impl) Loggable() map[string]interface{} {
	blocks := make([]string, 0, len(m.blocks))
	for _, v := range m.blocks {
		blocks = append(blocks, v.Cid().String())
	}
	return map[string]interface{}{
		"blocks": blocks,
		"wants":  m.Wantlist(),
	}
}
