package epictest

import (
	"bytes"
	"testing"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/blocks"
	"github.com/jbenet/go-ipfs/core"
	mocknet "github.com/jbenet/go-ipfs/p2p/net/mock"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestBitswapWithoutRouting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const numPeers = 4

	// create network
	mn, err := mocknet.FullMeshLinked(ctx, numPeers)
	if err != nil {
		t.Fatal(errors.Wrap(err))
	}

	peers := mn.Peers()
	if len(peers) < numPeers {
		t.Fatal(errors.New("test initialization error"))
	}

	// set the routing latency to infinity.
	conf := testutil.LatencyConfig{RoutingLatency: (525600 * time.Minute)}

	var nodes []*core.IpfsNode
	for _, p := range peers {
		n, err := core.NewIPFSNode(ctx, core.ConfigOption(MocknetTestRepo(p, mn.Host(p), conf)))
		if err != nil {
			t.Fatal(err)
		}
		defer n.Close()
		nodes = append(nodes, n)
	}

	// connect them
	for _, n1 := range nodes {
		for _, n2 := range nodes {
			if n1 == n2 {
				continue
			}

			log.Debug("connecting to other hosts")
			p2 := n2.PeerHost.Peerstore().PeerInfo(n2.PeerHost.ID())
			if err := n1.PeerHost.Connect(ctx, p2); err != nil {
				t.Fatal(err)
			}
		}
	}

	// add blocks to each before
	log.Debug("adding block.")
	block0 := blocks.NewBlock([]byte("block0"))
	block1 := blocks.NewBlock([]byte("block1"))

	// put 1 before
	if err := nodes[0].Blockstore.Put(block0); err != nil {
		t.Fatal(err)
	}

	//  get it out.
	for i, n := range nodes {
		// skip first because block not in its exchange. will hang.
		if i == 0 {
			continue
		}

		log.Debugf("%d %s get block.", i, n.Identity)
		b, err := n.Exchange.GetBlock(ctx, block0.Key())
		if err != nil {
			t.Error(err)
		} else if !bytes.Equal(b.Data, block0.Data) {
			t.Error("byte comparison fail")
		} else {
			log.Debug("got block: %s", b.Key())
		}
	}

	// put 1 after
	if err := nodes[1].Blockstore.Put(block1); err != nil {
		t.Fatal(err)
	}

	//  get it out.
	for _, n := range nodes {
		b, err := n.Exchange.GetBlock(ctx, block1.Key())
		if err != nil {
			t.Error(err)
		} else if !bytes.Equal(b.Data, block1.Data) {
			t.Error("byte comparison fail")
		} else {
			log.Debug("got block: %s", b.Key())
		}
	}
}
