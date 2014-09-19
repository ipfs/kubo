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
	testnet "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestGetBlockTimeout(t *testing.T) {

	net := testnet.VirtualNetwork()
	rs := testnet.VirtualRoutingServer()

	self := session(net, rs, []byte("peer id"))

	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	block := testutil.NewBlockOrFail(t, "block")
	_, err := self.exchange.Block(ctx, block.Key())

	if err != context.DeadlineExceeded {
		t.Fatal("Expected DeadlineExceeded error")
	}
}

func TestProviderForKeyButNetworkCannotFind(t *testing.T) {

	net := testnet.VirtualNetwork()
	rs := testnet.VirtualRoutingServer()

	block := testutil.NewBlockOrFail(t, "block")
	rs.Announce(&peer.Peer{}, block.Key()) // but not on network

	solo := session(net, rs, []byte("peer id"))

	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	_, err := solo.exchange.Block(ctx, block.Key())

	if err != context.DeadlineExceeded {
		t.Fatal("Expected DeadlineExceeded error")
	}
}

// TestGetBlockAfterRequesting...

func TestGetBlockFromPeerAfterPeerAnnounces(t *testing.T) {

	net := testnet.VirtualNetwork()
	rs := testnet.VirtualRoutingServer()
	block := testutil.NewBlockOrFail(t, "block")

	hasBlock := session(net, rs, []byte("hasBlock"))

	if err := hasBlock.blockstore.Put(block); err != nil {
		t.Fatal(err)
	}
	if err := hasBlock.exchange.HasBlock(context.Background(), block); err != nil {
		t.Fatal(err)
	}

	wantsBlock := session(net, rs, []byte("wantsBlock"))

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

type testnetBitSwap struct {
	peer       *peer.Peer
	exchange   exchange.Interface
	blockstore bstore.Blockstore
}

func session(net testnet.Network, rs testnet.RoutingServer, id peer.ID) testnetBitSwap {
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

func TestSendToWantingPeer(t *testing.T) {
	t.Log("Peer |w| tells me it wants file, but I don't have it")
	t.Log("Then another peer |o| sends it to me")
	t.Log("After receiving the file from |o|, I send it to the wanting peer |w|")
}
