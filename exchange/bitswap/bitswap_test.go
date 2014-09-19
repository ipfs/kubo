package bitswap

import (
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
	ipfs := session(net, rs, []byte("peer id"))
	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	block := testutil.NewBlockOrFail(t, "block")

	_, err := ipfs.exchange.Block(ctx, block.Key())
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
	t.Skip("Failing. Work in progress")

	net := testnet.VirtualNetwork()
	rs := testnet.VirtualRoutingServer()
	block := testutil.NewBlockOrFail(t, "block")

	hasBlock := session(net, rs, []byte("hasBlock"))

	rs.Announce(hasBlock.peer, block.Key())
	hasBlock.blockstore.Put(block)
	hasBlock.exchange.HasBlock(context.Background(), block)

	wantsBlock := session(net, rs, []byte("wantsBlock"))

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	_, err := wantsBlock.exchange.Block(ctx, block.Key())
	if err != nil {
		t.Log(err)
		t.Fatal("Expected to succeed")
	}
}

type ipfs struct {
	peer       *peer.Peer
	exchange   exchange.Interface
	blockstore bstore.Blockstore
}

func session(net testnet.Network, rs testnet.RoutingServer, id peer.ID) ipfs {
	p := &peer.Peer{}

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
	return ipfs{
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
