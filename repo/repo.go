package repo

import (
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	config "github.com/jbenet/go-ipfs/config"
	ic "github.com/jbenet/go-ipfs/crypto"
	epictest "github.com/jbenet/go-ipfs/epictest"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	namesys "github.com/jbenet/go-ipfs/namesys"
	inet "github.com/jbenet/go-ipfs/net"
	net "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	datastore2 "github.com/jbenet/go-ipfs/util/datastore2"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
	delay "github.com/jbenet/go-ipfs/util/delay"

	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

const kSizeBlockstoreWriteCache = 100
const IpnsValidatorTag = "ipns"

// If error is nil, all getters will return non-nil values
type RepoConfig func(ctx context.Context) (Repo, error)

// Repo owns its components. Callers must not Close them.
type Repo interface {

	// Getters

	ID() peer.ID
	Blockstore() blockstore.Blockstore
	Datastore() datastore2.ThreadSafeDatastoreCloser // TODO rename IpfsDatastore since we're going to keep adding datastore functionality (like batching, querying)
	Exchange() exchange.Interface
	Network() net.Network
	Peerstore() peer.Peerstore
	PrivateKey() ic.PrivKey //  TODO absolutely necessary?
	Routing() *dht.IpfsDHT  // TODO return interface

	// Behaviors

	OnlineMode() bool
	Bootstrap(ctx context.Context, peer peer.ID) error

	// Close closes all Repo components.
	Close() error
}

func Online(cfg *config.Config) RepoConfig {
	return func(parent context.Context) (Repo, error) {
		ctx, cancel := context.WithCancel(parent)
		success := false // flip to true after all sub-system inits succeed
		defer func() {
			if !success {
				cancel()
			}
		}()

		if cfg == nil {
			return nil, debugerror.Errorf("configuration required")
		}
		ds, err := makeDatastore(cfg.Datastore)
		if err != nil {
			return nil, debugerror.Wrap(err)
		}
		id, pk, err := initIdentity(&cfg.Identity, true) // TODO remove online param?
		if err != nil {
			return nil, debugerror.Wrap(err)
		}

		blockstore, err := bstore.WriteCached(bstore.NewBlockstore(ds), kSizeBlockstoreWriteCache)
		if err != nil {
			return nil, debugerror.Wrap(err)
		}

		// setup the network
		listenAddrs, err := listenAddresses(cfg)
		if err != nil {
			return nil, debugerror.Wrap(err)
		}
		ps := peer.NewPeerstore()
		n, err := inet.NewNetwork(ctx, listenAddrs, id, ps)
		if err != nil {
			return nil, debugerror.Wrap(err)
		}
		// Explicitly set these as our listen addrs. Why not do it inside
		// inet.NewNetwork? because this way we can listen on addresses without
		// necessarily advertising those publicly.
		addrs, err := n.InterfaceListenAddresses()
		if err != nil {
			return nil, debugerror.Wrap(err)
		}
		n.Peerstore().AddAddresses(id, addrs)
		n.Peerstore().AddPrivKey(id, pk)

		dhtRouting := dht.NewDHT(ctx, id, n, ds)
		dhtRouting.Validators[IpnsValidatorTag] = namesys.ValidateIpnsRecord
		const alwaysSendToPeer = true // use YesManStrategy
		bitswapNetwork := bsnet.NewFromIpfsNetwork(n, dhtRouting)
		exch := bitswap.New(ctx, id, bitswapNetwork, blockstore, alwaysSendToPeer)

		// TODO consider moving connection supervision into the Network. We've
		// discussed improvements to this Node constructor. One improvement
		// would be to make the node configurable, allowing clients to inject
		// an Exchange, Network, or Routing component and have the constructor
		// manage the wiring. In that scenario, this dangling function is a bit
		// awkward.
		go superviseConnections(ctx, n, dhtRouting, n.Peerstore(), cfg.Bootstrap)

		success = true
		return &repo{
			bitSwapNetwork: bitswapNetwork,
			blockstore:     blockstore,
			datastore:      ds,
			dht:            dhtRouting,
			exchange:       exch,
			id:             id,
			network:        n,
		}, nil
	}
}

// MocknetTestRepo belongs in the epictest/integration test package
func MocknetTestRepo(p peer.ID, n net.Network, conf epictest.Config) RepoConfig {
	return func(ctx context.Context) (Repo, error) {
		const kWriteCacheElems = 100
		const alwaysSendToPeer = true
		dsDelay := delay.Fixed(conf.BlockstoreLatency)
		ds := datastore2.CloserWrap(sync.MutexWrap(datastore2.WithDelay(datastore.NewMapDatastore(), dsDelay)))
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
	datastore      datastore2.ThreadSafeDatastoreCloser
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

func (r *repo) Datastore() datastore2.ThreadSafeDatastoreCloser {
	return r.datastore
}

func (r *repo) Blockstore() blockstore.Blockstore {
	return r.blockstore
}

func (r *repo) Exchange() exchange.Interface {
	return r.exchange
}

func (r *repo) PrivateKey() ic.PrivKey {
	return nil // TODO
}

func (r *repo) Peerstore() peer.Peerstore {
	return r.network.Peerstore()
}

func (r *repo) OnlineMode() bool {
	return true
}

func (r *repo) Network() net.Network {
	return r.network
}

func (r *repo) Close() error {
	// TODO Check errors, use CtxG
	r.Exchange().Close()
	r.Network().Close()
	r.Routing().Close()
	return nil // TODO
}

func (r *repo) Routing() *dht.IpfsDHT {
	return r.dht
}

func initIdentity(cfg *config.Identity, online bool) (peer.ID, ic.PrivKey, error) {

	if cfg.PeerID == "" {
		return "", nil, debugerror.New("Identity was not set in config (was ipfs init run?)")
	}

	if len(cfg.PeerID) == 0 {
		return "", nil, debugerror.New("No peer ID in config! (was ipfs init run?)")
	}

	id := peer.ID(b58.Decode(cfg.PeerID))

	// when not online, don't need to parse private keys (yet)
	if !online {
		return id, nil, nil
	}

	sk, err := loadPrivateKey(cfg, id)
	if err != nil {
		return "", nil, err
	}

	return id, sk, nil
}

func loadPrivateKey(cfg *config.Identity, id peer.ID) (ic.PrivKey, error) {
	sk, err := cfg.DecodePrivateKey("passphrase todo!")
	if err != nil {
		return nil, err
	}

	id2, err := peer.IDFromPrivateKey(sk)
	if err != nil {
		return nil, err
	}

	if id2 != id {
		return nil, fmt.Errorf("private key in config does not match id: %s != %s", id, id2)
	}

	return sk, nil
}

func listenAddresses(cfg *config.Config) ([]ma.Multiaddr, error) {

	var err error
	listen := make([]ma.Multiaddr, len(cfg.Addresses.Swarm))
	for i, addr := range cfg.Addresses.Swarm {

		listen[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("Failure to parse config.Addresses.Swarm[%d]: %s", i, cfg.Addresses.Swarm)
		}
	}

	return listen, nil
}
