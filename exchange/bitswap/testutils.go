package bitswap

import (
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ds_sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	exchange "github.com/jbenet/go-ipfs/exchange"
	tn "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	peer "github.com/jbenet/go-ipfs/peer"
	mockrouting "github.com/jbenet/go-ipfs/routing/mock"
	datastore2 "github.com/jbenet/go-ipfs/util/datastore2"
	delay "github.com/jbenet/go-ipfs/util/delay"
)

func NewSessionGenerator(
	net tn.Network, rs mockrouting.Server) SessionGenerator {
	return SessionGenerator{
		net: net,
		rs:  rs,
		ps:  peer.NewPeerstore(),
		seq: 0,
	}
}

type SessionGenerator struct {
	seq int
	net tn.Network
	rs  mockrouting.Server
	ps  peer.Peerstore
}

func (g *SessionGenerator) Next() Instance {
	g.seq++
	return session(g.net, g.rs, g.ps, []byte(string(g.seq)))
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
	blockstore blockstore.Blockstore

	blockstoreDelay delay.D
}

func (i *Instance) Blockstore() blockstore.Blockstore {
	return i.blockstore
}

func (i *Instance) SetBlockstoreLatency(t time.Duration) time.Duration {
	return i.blockstoreDelay.Set(t)
}

// session creates a test bitswap session.
//
// NB: It's easy make mistakes by providing the same peer ID to two different
// sessions. To safeguard, use the SessionGenerator to generate sessions. It's
// just a much better idea.
func session(net tn.Network, rs mockrouting.Server, ps peer.Peerstore, id peer.ID) Instance {
	p := ps.WithID(id)

	adapter := net.Adapter(p)
	htc := rs.Client(p)

	bsdelay := delay.Fixed(0)
	const kWriteCacheElems = 100
	bstore, err := blockstore.WriteCached(blockstore.NewBlockstore(ds_sync.MutexWrap(datastore2.WithDelay(ds.NewMapDatastore(), bsdelay))), kWriteCacheElems)
	if err != nil {
		// FIXME perhaps change signature and return error.
		panic(err.Error())
	}

	const alwaysSendToPeer = true
	ctx := context.TODO()

	bs := New(ctx, p, adapter, htc, bstore, alwaysSendToPeer)

	return Instance{
		Peer:            p,
		Exchange:        bs,
		blockstore:      bstore,
		blockstoreDelay: bsdelay,
	}
}
