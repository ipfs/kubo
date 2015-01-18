package epictest

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	core "github.com/jbenet/go-ipfs/core"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	host "github.com/jbenet/go-ipfs/p2p/host"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	delay "github.com/jbenet/go-ipfs/thirdparty/delay"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	"github.com/jbenet/go-ipfs/util/datastore2"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

var log = eventlog.Logger("epictest")

func MocknetTestRepo(p peer.ID, h host.Host, conf testutil.LatencyConfig) core.ConfigOption {
	return func(ctx context.Context) (*core.IpfsNode, error) {
		const kWriteCacheElems = 100
		const alwaysSendToPeer = true
		dsDelay := delay.Fixed(conf.BlockstoreLatency)
		ds := datastore2.CloserWrap(sync.MutexWrap(datastore2.WithDelay(datastore.NewMapDatastore(), dsDelay)))

		log.Debugf("MocknetTestRepo: %s %s %s", p, h.ID(), h)
		dhtt := dht.NewDHT(ctx, h, ds)
		bsn := bsnet.NewFromIpfsHost(h, dhtt)
		bstore, err := blockstore.WriteCached(blockstore.NewBlockstore(ds), kWriteCacheElems)
		if err != nil {
			return nil, err
		}
		exch := bitswap.New(ctx, p, bsn, bstore, alwaysSendToPeer)
		return &core.IpfsNode{
			Peerstore:  h.Peerstore(),
			Blockstore: bstore,
			Exchange:   exch,
			Datastore:  ds,
			PeerHost:   h,
			Routing:    dhtt,
			Identity:   p,
			DHT:        dhtt,
		}, nil
	}
}
