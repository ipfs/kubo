package message

import (
	"io"

	blocks "github.com/ipfs/go-ipfs/blocks"
	pb "github.com/ipfs/go-ipfs/exchange/bitswap/message/internal/pb"
	wantlist "github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	inet "github.com/ipfs/go-ipfs/p2p/net"
	u "github.com/ipfs/go-ipfs/util"

	ggio "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/io"
	proto "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
)

// TODO move message.go into the bitswap package
// TODO move bs/msg/internal/pb to bs/internal/pb and rename pb package to bitswap_pb

type BitSwapMessage interface {
	// Wantlist returns a slice of unique keys that represent data wanted by
	// the sender.
	Wantlist() []Entry

	// Blocks returns a slice of unique blocks
	Blocks() []*blocks.Block

	// AddEntry adds an entry to the Wantlist.
	AddEntry(key u.Key, priority int)

	Cancel(key u.Key)

	Empty() bool

	// Sets whether or not the contained wantlist represents the entire wantlist
	// true = full wantlist
	// false = wantlist 'patch'
	// default: true
	SetFull(isFull bool)

	Full() bool

	AddBlock(*blocks.Block)
	Exportable

	Loggable() map[string]interface{}
}

type Exportable interface {
	ToProto() *pb.Message
	ToNet(w io.Writer) error
}

type impl struct {
	full     bool
	wantlist map[u.Key]Entry
	blocks   map[u.Key]*blocks.Block
}

func New() BitSwapMessage {
	return newMsg()
}

func newMsg() *impl {
	return &impl{
		blocks:   make(map[u.Key]*blocks.Block),
		wantlist: make(map[u.Key]Entry),
		full:     true,
	}
}

type Entry struct {
	wantlist.Entry
	Cancel bool
}

func newMessageFromProto(pbm pb.Message) BitSwapMessage {
	m := newMsg()
	m.SetFull(pbm.GetWantlist().GetFull())
	for _, e := range pbm.GetWantlist().GetEntries() {
		m.addEntry(u.Key(e.GetBlock()), int(e.GetPriority()), e.GetCancel())
	}
	for _, d := range pbm.GetBlocks() {
		b := blocks.NewBlock(d)
		m.AddBlock(b)
	}
	return m
}

func (m *impl) SetFull(full bool) {
	m.full = full
}

func (m *impl) Full() bool {
	return m.full
}

func (m *impl) Empty() bool {
	return len(m.blocks) == 0 && len(m.wantlist) == 0
}

func (m *impl) Wantlist() []Entry {
	var out []Entry
	for _, e := range m.wantlist {
		out = append(out, e)
	}
	return out
}

func (m *impl) Blocks() []*blocks.Block {
	bs := make([]*blocks.Block, 0, len(m.blocks))
	for _, block := range m.blocks {
		bs = append(bs, block)
	}
	return bs
}

func (m *impl) Cancel(k u.Key) {
	delete(m.wantlist, k)
	m.addEntry(k, 0, true)
}

func (m *impl) AddEntry(k u.Key, priority int) {
	m.addEntry(k, priority, false)
}

func (m *impl) addEntry(k u.Key, priority int, cancel bool) {
	e, exists := m.wantlist[k]
	if exists {
		e.Priority = priority
		e.Cancel = cancel
	} else {
		m.wantlist[k] = Entry{
			Entry: wantlist.Entry{
				Key:      k,
				Priority: priority,
			},
			Cancel: cancel,
		}
	}
}

func (m *impl) AddBlock(b *blocks.Block) {
	m.blocks[b.Key()] = b
}

func FromNet(r io.Reader) (BitSwapMessage, error) {
	pbr := ggio.NewDelimitedReader(r, inet.MessageSizeMax)

	pb := new(pb.Message)
	if err := pbr.ReadMsg(pb); err != nil {
		return nil, err
	}

	m := newMessageFromProto(*pb)
	return m, nil
}

func (m *impl) ToProto() *pb.Message {
	pbm := new(pb.Message)
	pbm.Wantlist = new(pb.Message_Wantlist)
	for _, e := range m.wantlist {
		pbm.Wantlist.Entries = append(pbm.Wantlist.Entries, &pb.Message_Wantlist_Entry{
			Block:    proto.String(string(e.Key)),
			Priority: proto.Int32(int32(e.Priority)),
			Cancel:   proto.Bool(e.Cancel),
		})
	}
	for _, b := range m.Blocks() {
		pbm.Blocks = append(pbm.Blocks, b.Data)
	}
	return pbm
}

func (m *impl) ToNet(w io.Writer) error {
	pbw := ggio.NewDelimitedWriter(w)

	if err := pbw.WriteMsg(m.ToProto()); err != nil {
		return err
	}
	return nil
}

func (m *impl) Loggable() map[string]interface{} {
	var blocks []string
	for _, v := range m.blocks {
		blocks = append(blocks, v.Key().Pretty())
	}
	return map[string]interface{}{
		"blocks": blocks,
		"wants":  m.Wantlist(),
	}
}
