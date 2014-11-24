package bitswap

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ds_sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/jbenet/go-ipfs/blocks"
	"github.com/jbenet/go-ipfs/blocks/blockstore"
	"github.com/jbenet/go-ipfs/exchange"
	tn "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	"github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/routing/mock"
)

/*
TODO: This whole file needs somewhere better to live.
The issue is that its very difficult to move it somewhere else
without creating circular dependencies.
Additional thought required.
*/

func NewBlockGenerator() BlockGenerator {
	return BlockGenerator{}
}

type BlockGenerator struct {
	seq int
}

func (bg *BlockGenerator) Next() *blocks.Block {
	bg.seq++
	return blocks.NewBlock([]byte(string(bg.seq)))
}

func (bg *BlockGenerator) Blocks(n int) []*blocks.Block {
	blocks := make([]*blocks.Block, 0)
	for i := 0; i < n; i++ {
		b := bg.Next()
		blocks = append(blocks, b)
	}
	return blocks
}

func NewSessionGenerator(
	net tn.Network, rs mock.RoutingServer) SessionGenerator {
	return SessionGenerator{
		net: net,
		rs:  rs,
		seq: 0,
	}
}

type SessionGenerator struct {
	seq int
	net tn.Network
	rs  mock.RoutingServer
}

func (g *SessionGenerator) Next() Instance {
	g.seq++
	return session(g.net, g.rs, []byte(string(g.seq)))
}

func (g *SessionGenerator) Instances(n int) []Instance {
	instances := make([]Instance, 0)
	for j := 0; j < n; j++ {
		inst := g.Next()
		instances = append(instances, inst)
	}
	return instances
}

type Instance struct {
	Peer       peer.Peer
	Exchange   exchange.Interface
	Blockstore blockstore.Blockstore
}

// session creates a test bitswap session.
//
// NB: It's easy make mistakes by providing the same peer ID to two different
// sessions. To safeguard, use the SessionGenerator to generate sessions. It's
// just a much better idea.
func session(net tn.Network, rs mock.RoutingServer, id peer.ID) Instance {
	p := peer.WithID(id)

	adapter := net.Adapter(p)
	htc := rs.Client(p)
	bstore := blockstore.NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))

	const alwaysSendToPeer = true
	ctx := context.TODO()

	bs := New(ctx, p, adapter, htc, bstore, alwaysSendToPeer)

	return Instance{
		Peer:       p,
		Exchange:   bs,
		Blockstore: bstore,
	}
}
