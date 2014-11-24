package bitswap

import (
	"bytes"
	"sync"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/blocks"
	tn "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	peer "github.com/jbenet/go-ipfs/peer"
	mock "github.com/jbenet/go-ipfs/routing/mock"
)

func TestClose(t *testing.T) {
	// TODO
	t.Skip("TODO Bitswap's Close implementation is a WIP")
	vnet := tn.VirtualNetwork()
	rout := mock.VirtualRoutingServer()
	sesgen := NewSessionGenerator(vnet, rout)
	bgen := NewBlockGenerator()

	block := bgen.Next()
	bitswap := sesgen.Next()

	bitswap.Exchange.Close()
	bitswap.Exchange.GetBlock(context.Background(), block.Key())
}

func TestGetBlockTimeout(t *testing.T) {

	net := tn.VirtualNetwork()
	rs := mock.VirtualRoutingServer()
	g := NewSessionGenerator(net, rs)

	self := g.Next()

	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	block := blocks.NewBlock([]byte("block"))
	_, err := self.Exchange.GetBlock(ctx, block.Key())

	if err != context.DeadlineExceeded {
		t.Fatal("Expected DeadlineExceeded error")
	}
}

func TestProviderForKeyButNetworkCannotFind(t *testing.T) {

	net := tn.VirtualNetwork()
	rs := mock.VirtualRoutingServer()
	g := NewSessionGenerator(net, rs)

	block := blocks.NewBlock([]byte("block"))
	rs.Announce(peer.WithIDString("testing"), block.Key()) // but not on network

	solo := g.Next()

	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	_, err := solo.Exchange.GetBlock(ctx, block.Key())

	if err != context.DeadlineExceeded {
		t.Fatal("Expected DeadlineExceeded error")
	}
}

// TestGetBlockAfterRequesting...

func TestGetBlockFromPeerAfterPeerAnnounces(t *testing.T) {

	net := tn.VirtualNetwork()
	rs := mock.VirtualRoutingServer()
	block := blocks.NewBlock([]byte("block"))
	g := NewSessionGenerator(net, rs)

	hasBlock := g.Next()

	if err := hasBlock.Blockstore.Put(block); err != nil {
		t.Fatal(err)
	}
	if err := hasBlock.Exchange.HasBlock(context.Background(), block); err != nil {
		t.Fatal(err)
	}

	wantsBlock := g.Next()

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	received, err := wantsBlock.Exchange.GetBlock(ctx, block.Key())
	if err != nil {
		t.Log(err)
		t.Fatal("Expected to succeed")
	}

	if !bytes.Equal(block.Data, received.Data) {
		t.Fatal("Data doesn't match")
	}
}

func TestLargeSwarm(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()
	numInstances := 5
	numBlocks := 2
	PerformDistributionTest(t, numInstances, numBlocks)
}

func TestLargeFile(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	t.Parallel()
	numInstances := 10
	numBlocks := 100
	PerformDistributionTest(t, numInstances, numBlocks)
}

func PerformDistributionTest(t *testing.T, numInstances, numBlocks int) {
	if testing.Short() {
		t.SkipNow()
	}
	net := tn.VirtualNetwork()
	rs := mock.VirtualRoutingServer()
	sg := NewSessionGenerator(net, rs)
	bg := NewBlockGenerator()

	t.Log("Test a few nodes trying to get one file with a lot of blocks")

	instances := sg.Instances(numInstances)
	blocks := bg.Blocks(numBlocks)

	t.Log("Give the blocks to the first instance")

	first := instances[0]
	for _, b := range blocks {
		first.Blockstore.Put(b)
		first.Exchange.HasBlock(context.Background(), b)
		rs.Announce(first.Peer, b.Key())
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
			if _, err := inst.Blockstore.Get(b.Key()); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func getOrFail(bitswap Instance, b *blocks.Block, t *testing.T, wg *sync.WaitGroup) {
	if _, err := bitswap.Blockstore.Get(b.Key()); err != nil {
		_, err := bitswap.Exchange.GetBlock(context.Background(), b.Key())
		if err != nil {
			t.Fatal(err)
		}
	}
	wg.Done()
}

// TODO simplify this test. get to the _essence_!
func TestSendToWantingPeer(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	net := tn.VirtualNetwork()
	rs := mock.VirtualRoutingServer()
	sg := NewSessionGenerator(net, rs)
	bg := NewBlockGenerator()

	me := sg.Next()
	w := sg.Next()
	o := sg.Next()

	t.Logf("Session %v\n", me.Peer)
	t.Logf("Session %v\n", w.Peer)
	t.Logf("Session %v\n", o.Peer)

	alpha := bg.Next()

	const timeout = 100 * time.Millisecond // FIXME don't depend on time

	t.Logf("Peer %v attempts to get %v. NB: not available\n", w.Peer, alpha.Key())
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	_, err := w.Exchange.GetBlock(ctx, alpha.Key())
	if err == nil {
		t.Fatalf("Expected %v to NOT be available", alpha.Key())
	}

	beta := bg.Next()
	t.Logf("Peer %v announes availability  of %v\n", w.Peer, beta.Key())
	ctx, _ = context.WithTimeout(context.Background(), timeout)
	if err := w.Blockstore.Put(beta); err != nil {
		t.Fatal(err)
	}
	w.Exchange.HasBlock(ctx, beta)

	t.Logf("%v gets %v from %v and discovers it wants %v\n", me.Peer, beta.Key(), w.Peer, alpha.Key())
	ctx, _ = context.WithTimeout(context.Background(), timeout)
	if _, err := me.Exchange.GetBlock(ctx, beta.Key()); err != nil {
		t.Fatal(err)
	}

	t.Logf("%v announces availability of %v\n", o.Peer, alpha.Key())
	ctx, _ = context.WithTimeout(context.Background(), timeout)
	if err := o.Blockstore.Put(alpha); err != nil {
		t.Fatal(err)
	}
	o.Exchange.HasBlock(ctx, alpha)

	t.Logf("%v requests %v\n", me.Peer, alpha.Key())
	ctx, _ = context.WithTimeout(context.Background(), timeout)
	if _, err := me.Exchange.GetBlock(ctx, alpha.Key()); err != nil {
		t.Fatal(err)
	}

	t.Logf("%v should now have %v\n", w.Peer, alpha.Key())
	block, err := w.Blockstore.Get(alpha.Key())
	if err != nil {
		t.Fatal("Should not have received an error")
	}
	if block.Key() != alpha.Key() {
		t.Fatal("Expected to receive alpha from me")
	}
}
