package bitswap

import (
	"bytes"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	bstore "github.com/jbenet/go-ipfs/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	notifications "github.com/jbenet/go-ipfs/exchange/bitswap/notifications"
	strategy "github.com/jbenet/go-ipfs/exchange/bitswap/strategy"
	tn "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	peer "github.com/jbenet/go-ipfs/peer"
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

func TestSendToWantingPeer(t *testing.T) {
	t.Log("I get a file from peer |w|. In this message, I receive |w|'s wants")
	t.Log("Peer |w| tells me it wants file |f|, but I don't have it")
	t.Log("Later, peer |o| sends |f| to me")
	t.Log("After receiving |f| from |o|, I send it to the wanting peer |w|")
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

func (g *SessionGenerator) Next() testnetBitSwap {
	g.seq++
	return session(g.net, g.rs, []byte(string(g.seq)))
}

type testnetBitSwap struct {
	peer       *peer.Peer
	exchange   exchange.Interface
	blockstore bstore.Blockstore
}

// session creates a test bitswap session.
//
// NB: It's easy make mistakes by providing the same peer ID to two different
// sessions. To safeguard, use the SessionGenerator to generate sessions. It's
// just a much better idea.
func session(net tn.Network, rs tn.RoutingServer, id peer.ID) testnetBitSwap {
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
	}
	adapter.SetDelegate(bs)
	return testnetBitSwap{
		peer:       p,
		exchange:   bs,
		blockstore: blockstore,
	}
}
