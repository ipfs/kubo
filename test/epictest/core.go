package epictest

import (
	"io"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	blockservice "github.com/jbenet/go-ipfs/blockservice"
	core "github.com/jbenet/go-ipfs/core"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	importer "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	host "github.com/jbenet/go-ipfs/p2p/host"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	path "github.com/jbenet/go-ipfs/path"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	util "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/datastore2"
	delay "github.com/jbenet/go-ipfs/util/delay"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

var log = eventlog.Logger("epictest")

// TODO merge with core.IpfsNode
type Core struct {
	*core.IpfsNode
}

func (c *Core) ID() peer.ID {
	return c.IpfsNode.Identity
}

func (c *Core) Bootstrap(ctx context.Context, p peer.PeerInfo) error {
	return c.IpfsNode.Bootstrap(ctx, []peer.PeerInfo{p})
}

func (c *Core) Cat(k util.Key) (io.Reader, error) {
	catterdag := c.IpfsNode.DAG
	nodeCatted, err := (&path.Resolver{catterdag}).ResolvePath(k.String())
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(nodeCatted, catterdag)
}

func (c *Core) Add(r io.Reader) (util.Key, error) {
	nodeAdded, err := importer.BuildDagFromReader(
		r,
		c.IpfsNode.DAG,
		nil,
		chunk.DefaultSplitter,
	)
	if err != nil {
		return "", err
	}
	return nodeAdded.Key()
}

func makeCore(ctx context.Context, rf RepoFactory) (*Core, error) {
	node, err := rf(ctx)
	if err != nil {
		return nil, err
	}

	node.Blocks, err = blockservice.New(node.Blockstore, node.Exchange)
	if err != nil {
		return nil, err
	}

	node.DAG = merkledag.NewDAGService(node.Blocks)
	// to make sure nothing is omitted, init each individual field and assign
	// all at once at the bottom.
	return &Core{
		IpfsNode: node,
	}, nil
}

type RepoFactory func(ctx context.Context) (*core.IpfsNode, error)

type Repo interface {
	ID() peer.ID
	Blockstore() blockstore.Blockstore
	Exchange() exchange.Interface

	Bootstrap(ctx context.Context, peer peer.ID) error
}

type repo struct {
	// DHT, Exchange, Network,Datastore
	bitSwapNetwork bsnet.BitSwapNetwork
	blockstore     blockstore.Blockstore
	exchange       exchange.Interface
	datastore      datastore.ThreadSafeDatastore
	host           host.Host
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

func MocknetTestRepo(p peer.ID, h host.Host, conf testutil.LatencyConfig) RepoFactory {
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
