package integrationtest

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	core "github.com/jbenet/go-ipfs/core"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	host "github.com/jbenet/go-ipfs/p2p/host"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	"github.com/jbenet/go-ipfs/repo"
	delay "github.com/jbenet/go-ipfs/thirdparty/delay"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

var log = eventlog.Logger("epictest")

func MocknetTestRepo(p peer.ID, h host.Host, conf testutil.LatencyConfig, routing core.RoutingOption) core.ConfigOption {
	return func(ctx context.Context) (*core.IpfsNode, error) {
		const kWriteCacheElems = 100
		const alwaysSendToPeer = true
		dsDelay := delay.Fixed(conf.BlockstoreLatency)
		r := &repo.Mock{
			D: ds2.CloserWrap(syncds.MutexWrap(ds2.WithDelay(datastore.NewMapDatastore(), dsDelay))),
		}
		ds := r.Datastore()

		n := &core.IpfsNode{
			Peerstore: h.Peerstore(),
			Repo:      r,
			PeerHost:  h,
			Identity:  p,
		}
		dhtt, err := routing(ctx, n)
		if err != nil {
			return nil, err
		}

		bsn := bsnet.NewFromIpfsHost(h, dhtt)
		bstore, err := blockstore.WriteCached(blockstore.NewBlockstore(ds), kWriteCacheElems)
		if err != nil {
			return nil, err
		}
		exch := bitswap.New(ctx, p, bsn, bstore, alwaysSendToPeer)
		n.Blockstore = bstore
		n.Exchange = exch
		n.Routing = dhtt
		return n, nil
	}
}
