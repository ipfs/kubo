package bitswap

import (
	"bytes"
	"sync"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	"github.com/jbenet/go-ipfs/blocks"
	bstore "github.com/jbenet/go-ipfs/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	notifications "github.com/jbenet/go-ipfs/exchange/bitswap/notifications"
	strategy "github.com/jbenet/go-ipfs/exchange/bitswap/strategy"
	tn "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	peer "github.com/jbenet/go-ipfs/peer"
	util "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestGetBlockTimeout(t *testing.T) {

	net := tn.VirtualNetwork()
	rs := tn.VirtualRoutingServer()
	g := NewSessionGenerator(net, rs)

	self := g.Next()

	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	block := testutil.NewBlockOrFail(t, "block")
	_, err := self.exchange.Block(ctx, block.Key())

	if err != context.DeadlineExceeded {
		t.Fatal("Expected DeadlineExceeded error")
	}
}

func TestProviderForKeyButNetworkCannotFind(t *testing.T) {

	net := tn.VirtualNetwork()
	rs := tn.VirtualRoutingServer()
	g := NewSessionGenerator(net, rs)

	block := testutil.NewBlockOrFail(t, "block")
	rs.Announce(&peer.Peer{}, block.Key()) // but not on network

	solo := g.Next()

	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	_, err := solo.exchange.Block(ctx, block.Key())

	if err != context.DeadlineExceeded {
		t.Fatal("Expected DeadlineExceeded error")
	}
}

// TestGetBlockAfterRequesting...

func TestGetBlockFromPeerAfterPeerAnnounces(t *testing.T) {

	net := tn.VirtualNetwork()
	rs := tn.VirtualRoutingServer()
	block := testutil.NewBlockOrFail(t, "block")
	g := NewSessionGenerator(net, rs)

	hasBlock := g.Next()

	if err := hasBlock.blockstore.Put(block); err != nil {
		t.Fatal(err)
	}
	if err := hasBlock.exchange.HasBlock(context.Background(), block); err != nil {
		t.Fatal(err)
	}

	wantsBlock := g.Next()

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	received, err := wantsBlock.exchange.Block(ctx, block.Key())
	if err != nil {
		t.Log(err)
		t.Fatal("Expected to succeed")
	}

	if !bytes.Equal(block.Data, received.Data) {
		t.Fatal("Data doesn't match")
	}
}

func TestSwarm(t *testing.T) {
	net := tn.VirtualNetwork()
	rs := tn.VirtualRoutingServer()
	sg := NewSessionGenerator(net, rs)
	bg := NewBlockGenerator(t)

	t.Log("Create a ton of instances, and just a few blocks")

	numInstances := 500
	numBlocks := 2

	instances := sg.Instances(numInstances)
	blocks := bg.Blocks(numBlocks)

	t.Log("Give the blocks to the first instance")

	first := instances[0]
	for _, b := range blocks {
		first.blockstore.Put(*b)
		first.exchange.HasBlock(context.Background(), *b)
		rs.Announce(first.peer, b.Key())
	}

	t.Log("Distribute!")

	var wg sync.WaitGroup

	for _, inst := range instances {
		for _, b := range blocks {
			wg.Add(1)
			// NB: executing getOrFail concurrently puts tremendous pressure on
			// the goroutine scheduler
			getOrFail(inst, b, t, &wg)
		}
	}
	wg.Wait()

	t.Log("Verify!")

	for _, inst := range instances {
		for _, b := range blocks {
			if _, err := inst.blockstore.Get(b.Key()); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func getOrFail(bitswap instance, b *blocks.Block, t *testing.T, wg *sync.WaitGroup) {
	if _, err := bitswap.blockstore.Get(b.Key()); err != nil {
		_, err := bitswap.exchange.Block(context.Background(), b.Key())
		if err != nil {
			t.Fatal(err)
		}
	}
	wg.Done()
}

// TODO simplify this test. get to the _essence_!
func TestSendToWantingPeer(t *testing.T) {
	util.Debug = true

	net := tn.VirtualNetwork()
	rs := tn.VirtualRoutingServer()
	sg := NewSessionGenerator(net, rs)
	bg := NewBlockGenerator(t)

	me := sg.Next()
	w := sg.Next()
	o := sg.Next()

	t.Logf("Session %v\n", me.peer.Key().Pretty())
	t.Logf("Session %v\n", w.peer.Key().Pretty())
	t.Logf("Session %v\n", o.peer.Key().Pretty())

	alpha := bg.Next()

	const timeout = 1 * time.Millisecond // FIXME don't depend on time

	t.Logf("Peer %v attempts to get %v. NB: not available\n", w.peer.Key().Pretty(), alpha.Key().Pretty())
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	_, err := w.exchange.Block(ctx, alpha.Key())
	if err == nil {
		t.Fatalf("Expected %v to NOT be available", alpha.Key().Pretty())
	}

	beta := bg.Next()
	t.Logf("Peer %v announes availability  of %v\n", w.peer.Key().Pretty(), beta.Key().Pretty())
	ctx, _ = context.WithTimeout(context.Background(), timeout)
	if err := w.blockstore.Put(beta); err != nil {
		t.Fatal(err)
	}
	w.exchange.HasBlock(ctx, beta)

	t.Logf("%v gets %v from %v and discovers it wants %v\n", me.peer.Key().Pretty(), beta.Key().Pretty(), w.peer.Key().Pretty(), alpha.Key().Pretty())
	ctx, _ = context.WithTimeout(context.Background(), timeout)
	if _, err := me.exchange.Block(ctx, beta.Key()); err != nil {
		t.Fatal(err)
	}

	t.Logf("%v announces availability of %v\n", o.peer.Key().Pretty(), alpha.Key().Pretty())
	ctx, _ = context.WithTimeout(context.Background(), timeout)
	if err := o.blockstore.Put(alpha); err != nil {
		t.Fatal(err)
	}
	o.exchange.HasBlock(ctx, alpha)

	t.Logf("%v requests %v\n", me.peer.Key().Pretty(), alpha.Key().Pretty())
	ctx, _ = context.WithTimeout(context.Background(), timeout)
	if _, err := me.exchange.Block(ctx, alpha.Key()); err != nil {
		t.Fatal(err)
	}

	t.Logf("%v should now have %v\n", w.peer.Key().Pretty(), alpha.Key().Pretty())
	block, err := w.blockstore.Get(alpha.Key())
	if err != nil {
		t.Fatal("Should not have received an error")
	}
	if block.Key() != alpha.Key() {
		t.Fatal("Expected to receive alpha from me")
	}
}

func NewBlockGenerator(t *testing.T) BlockGenerator {
	return BlockGenerator{
		T: t,
	}
}

type BlockGenerator struct {
	*testing.T // b/c block generation can fail
	seq        int
}

func (bg *BlockGenerator) Next() blocks.Block {
	bg.seq++
	return testutil.NewBlockOrFail(bg.T, string(bg.seq))
}

func (bg *BlockGenerator) Blocks(n int) []*blocks.Block {
	blocks := make([]*blocks.Block, 0)
	for i := 0; i < n; i++ {
		b := bg.Next()
		blocks = append(blocks, &b)
	}
	return blocks
}

func NewSessionGenerator(
	net tn.Network, rs tn.RoutingServer) SessionGenerator {
	return SessionGenerator{
		net: net,
		rs:  rs,
		seq: 0,
	}
}

type SessionGenerator struct {
	seq int
	net tn.Network
	rs  tn.RoutingServer
}

func (g *SessionGenerator) Next() instance {
	g.seq++
	return session(g.net, g.rs, []byte(string(g.seq)))
}

func (g *SessionGenerator) Instances(n int) []instance {
	instances := make([]instance, 0)
	for j := 0; j < n; j++ {
		inst := g.Next()
		instances = append(instances, inst)
	}
	return instances
}

type instance struct {
	peer       *peer.Peer
	exchange   exchange.Interface
	blockstore bstore.Blockstore
}

// session creates a test bitswap session.
//
// NB: It's easy make mistakes by providing the same peer ID to two different
// sessions. To safeguard, use the SessionGenerator to generate sessions. It's
// just a much better idea.
func session(net tn.Network, rs tn.RoutingServer, id peer.ID) instance {
	p := &peer.Peer{ID: id}

	adapter := net.Adapter(p)
	htc := rs.Client(p)

	blockstore := bstore.NewBlockstore(ds.NewMapDatastore())
	bs := &bitswap{
		blockstore:    blockstore,
		notifications: notifications.New(),
		strategy:      strategy.New(),
		routing:       htc,
		sender:        adapter,
		wantlist:      util.NewKeySet(),
	}
	adapter.SetDelegate(bs)
	return instance{
		peer:       p,
		exchange:   bs,
		blockstore: blockstore,
	}
}
