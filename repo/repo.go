package repo

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	epictest "github.com/jbenet/go-ipfs/epictest"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	net "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	datastore2 "github.com/jbenet/go-ipfs/util/datastore2"
	delay "github.com/jbenet/go-ipfs/util/delay"
)

type RepoConfig func(ctx context.Context) (Repo, error)

type Repo interface {
	ID() peer.ID
	Blockstore() blockstore.Blockstore
	Exchange() exchange.Interface
	Peerstore() peer.Peerstore

	Bootstrap(ctx context.Context, peer peer.ID) error
}

// MocknetTestRepo belongs in the epictest/integration test package
func MocknetTestRepo(p peer.ID, n net.Network, conf epictest.Config) RepoConfig {
	return func(ctx context.Context) (Repo, error) {
		const kWriteCacheElems = 100
		const alwaysSendToPeer = true
		dsDelay := delay.Fixed(conf.BlockstoreLatency)
		ds := sync.MutexWrap(datastore2.WithDelay(datastore.NewMapDatastore(), dsDelay))
		dhtt := dht.NewDHT(ctx, p, n, ds)
		bsn := bsnet.NewFromIpfsNetwork(n, dhtt)
		bstore, err := blockstore.WriteCached(blockstore.NewBlockstore(ds), kWriteCacheElems)
		if err != nil {
			return nil, err
		}
		exch := bitswap.New(ctx, p, bsn, bstore, alwaysSendToPeer)
		return &repo{
			bitSwapNetwork: bsn,
			blockstore:     bstore,
			datastore:      ds,
			dht:            dhtt,
			exchange:       exch,
			id:             p,
			network:        n,
		}, nil
	}
}

type repo struct {
	bitSwapNetwork bsnet.BitSwapNetwork
	blockstore     blockstore.Blockstore
	exchange       exchange.Interface
	datastore      datastore.ThreadSafeDatastore
	network        net.Network
	dht            *dht.IpfsDHT
	id             peer.ID
}

func (r *repo) ID() peer.ID {
	return r.id
}

func (c *repo) Bootstrap(ctx context.Context, p peer.ID) error {
	return c.dht.Connect(ctx, p)
}

func (r *repo) Datastore() datastore.ThreadSafeDatastore {
	return r.datastore
}

func (r *repo) Blockstore() blockstore.Blockstore {
	return r.blockstore
}

func (r *repo) Exchange() exchange.Interface {
	return r.exchange
}

func (r *repo) Peerstore() peer.Peerstore {
	return r.network.Peerstore()
}
