package core

import (
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	config "github.com/jbenet/go-ipfs/config"
	diag "github.com/jbenet/go-ipfs/diagnostics"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	offline "github.com/jbenet/go-ipfs/exchange/offline"
	mount "github.com/jbenet/go-ipfs/fuse/mount"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	namesys "github.com/jbenet/go-ipfs/namesys"
	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	p2phost "github.com/jbenet/go-ipfs/p2p/host"
	p2pbhost "github.com/jbenet/go-ipfs/p2p/host/basic"
	swarm "github.com/jbenet/go-ipfs/p2p/net/swarm"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	path "github.com/jbenet/go-ipfs/path"
	pin "github.com/jbenet/go-ipfs/pin"
	routing "github.com/jbenet/go-ipfs/routing"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	util "github.com/jbenet/go-ipfs/util"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"
)

const IpnsValidatorTag = "ipns"
const kSizeBlockstoreWriteCache = 100

var log = eventlog.Logger("core")

type mode int

const (
	// zero value is not a valid mode, must be explicitly set
	invalidMode mode = iota
	offlineMode
	onlineMode
)

// IpfsNode is IPFS Core module. It represents an IPFS instance.
type IpfsNode struct {

	// Self
	Identity peer.ID // the local node's identity

	// TODO abstract as repo.Repo
	Config    *config.Config                // the node's configuration
	Datastore ds2.ThreadSafeDatastoreCloser // the local datastore

	// Local node
	Pinning pin.Pinner // the pinning manager
	Mounts  Mounts     // current mount state, if any.

	// Services
	Peerstore  peer.Peerstore       // storage for other Peer instances
	Blockstore bstore.Blockstore    // the block store (lower level)
	Blocks     *bserv.BlockService  // the block service, get/add blocks.
	DAG        merkledag.DAGService // the merkle dag service, get/add objects.
	Resolver   *path.Resolver       // the path resolution system

	// Online
	PrivateKey  ic.PrivKey          // the local node's private Key
	PeerHost    p2phost.Host        // the network host (server+client)
	Routing     routing.IpfsRouting // the routing system. recommend ipfs-dht
	Exchange    exchange.Interface  // the block exchange + strategy (bitswap)
	Namesys     namesys.NameSystem  // the name system, resolves paths to hashes
	Diagnostics *diag.Diagnostics   // the diagnostics service

	ctxgroup.ContextGroup

	// dht allows node to Bootstrap when dht is present
	// TODO privatize before merging. This is here temporarily during the
	// migration of the TestNet constructor
	DHT  *dht.IpfsDHT
	mode mode
}

// Mounts defines what the node's mount state is. This should
// perhaps be moved to the daemon or mount. It's here because
// it needs to be accessible across daemon requests.
type Mounts struct {
	Ipfs mount.Mount
	Ipns mount.Mount
}

type ConfigOption func(ctx context.Context) (*IpfsNode, error)

func NewIPFSNode(ctx context.Context, option ConfigOption) (*IpfsNode, error) {
	node, err := option(ctx)
	if err != nil {
		return nil, err
	}

	// Need to make sure it's perfectly clear 1) which variables are expected
	// to be initialized at this point, and 2) which variables will be
	// initialized after this point.

	node.Blocks, err = bserv.New(node.Blockstore, node.Exchange)
	if err != nil {
		return nil, debugerror.Wrap(err)
	}
	if node.Peerstore == nil {
		node.Peerstore = peer.NewPeerstore()
	}
	node.DAG = merkledag.NewDAGService(node.Blocks)
	node.Pinning, err = pin.LoadPinner(node.Datastore, node.DAG)
	if err != nil {
		node.Pinning = pin.NewPinner(node.Datastore, node.DAG)
	}
	node.Resolver = &path.Resolver{DAG: node.DAG}
	return node, nil
}

func Offline(cfg *config.Config) ConfigOption {
	return Standard(cfg, false)
}

func Online(cfg *config.Config) ConfigOption {
	return Standard(cfg, true)
}

// DEPRECATED: use Online, Offline functions
func Standard(cfg *config.Config, online bool) ConfigOption {
	return func(ctx context.Context) (n *IpfsNode, err error) {

		success := false // flip to true after all sub-system inits succeed
		defer func() {
			if !success && n != nil {
				n.Close()
			}
		}()

		if cfg == nil {
			return nil, debugerror.Errorf("configuration required")
		}
		n = &IpfsNode{
			mode: func() mode {
				if online {
					return onlineMode
				}
				return offlineMode
			}(),
			Config: cfg,
		}

		n.ContextGroup = ctxgroup.WithContextAndTeardown(ctx, n.teardown)
		ctx = n.ContextGroup.Context()

		// setup Peerstore
		n.Peerstore = peer.NewPeerstore()

		// setup datastore.
		if n.Datastore, err = makeDatastore(cfg.Datastore); err != nil {
			return nil, debugerror.Wrap(err)
		}

		// setup local peer ID (private key is loaded in online setup)
		if err := n.loadID(); err != nil {
			return nil, err
		}

		n.Blockstore, err = bstore.WriteCached(bstore.NewBlockstore(n.Datastore), kSizeBlockstoreWriteCache)
		if err != nil {
			return nil, debugerror.Wrap(err)
		}

		// setup online services
		if online {
			if err := n.StartOnlineServices(); err != nil {
				return nil, err // debugerror.Wraps.
			}
		} else {
			n.Exchange = offline.Exchange(n.Blockstore)
		}

		success = true
		return n, nil
	}
}

func (n *IpfsNode) StartOnlineServices() error {
	ctx := n.Context()

	if n.PeerHost != nil { // already online.
		return debugerror.New("node already online")
	}

	// load private key
	if err := n.loadPrivateKey(); err != nil {
		return err
	}

	peerhost, err := constructPeerHost(ctx, n.ContextGroup, n.Config, n.Identity, n.Peerstore)
	if err != nil {
		return debugerror.Wrap(err)
	}
	n.PeerHost = peerhost

	// setup diagnostics service
	n.Diagnostics = diag.NewDiagnostics(n.Identity, n.PeerHost)

	// setup routing service
	dhtRouting, err := constructDHTRouting(ctx, n.ContextGroup, n.PeerHost, n.Datastore)
	if err != nil {
		return debugerror.Wrap(err)
	}
	n.DHT = dhtRouting
	n.Routing = dhtRouting

	// setup exchange service
	const alwaysSendToPeer = true // use YesManStrategy
	bitswapNetwork := bsnet.NewFromIpfsHost(n.PeerHost, n.Routing)
	n.Exchange = bitswap.New(ctx, n.Identity, bitswapNetwork, n.Blockstore, alwaysSendToPeer)

	// setup name system
	// TODO implement an offline namesys that serves only local names.
	n.Namesys = namesys.NewNameSystem(n.Routing)

	// TODO consider moving connection supervision into the Network. We've
	// discussed improvements to this Node constructor. One improvement
	// would be to make the node configurable, allowing clients to inject
	// an Exchange, Network, or Routing component and have the constructor
	// manage the wiring. In that scenario, this dangling function is a bit
	// awkward.
	var bootstrapPeers []peer.PeerInfo
	for _, bootstrap := range n.Config.Bootstrap {
		p, err := toPeer(bootstrap)
		if err != nil {
			log.Event(ctx, "bootstrapError", n.Identity, lgbl.Error(err))
			log.Errorf("%s bootstrap error: %s", n.Identity, err)
			return err
		}
		bootstrapPeers = append(bootstrapPeers, p)
	}
	go superviseConnections(ctx, n.PeerHost, n.DHT, n.Peerstore, bootstrapPeers)
	return nil
}

func (n *IpfsNode) teardown() error {
	if err := n.Datastore.Close(); err != nil {
		return err
	}
	return nil
}

func (n *IpfsNode) OnlineMode() bool {
	switch n.mode {
	case onlineMode:
		return true
	default:
		return false
	}
}

func (n *IpfsNode) Resolve(k util.Key) (*merkledag.Node, error) {
	return (&path.Resolver{n.DAG}).ResolvePath(k.String())
}

func (n *IpfsNode) Bootstrap(ctx context.Context, peers []peer.PeerInfo) error {
	if n.DHT != nil {
		for _, p := range peers {
			// TODO bootstrap(ctx, n.PeerHost, n.DHT, n.Peerstore, peers)
			if err := n.DHT.Connect(ctx, p.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *IpfsNode) loadID() error {
	if n.Identity != "" {
		return debugerror.New("identity already loaded")
	}

	cid := n.Config.Identity.PeerID
	if cid == "" {
		return debugerror.New("Identity was not set in config (was ipfs init run?)")
	}
	if len(cid) == 0 {
		return debugerror.New("No peer ID in config! (was ipfs init run?)")
	}

	n.Identity = peer.ID(b58.Decode(cid))
	return nil
}

func (n *IpfsNode) loadPrivateKey() error {
	if n.Identity == "" || n.Peerstore == nil {
		return debugerror.New("loaded private key out of order.")
	}

	if n.PrivateKey != nil {
		return debugerror.New("private key already loaded")
	}

	sk, err := loadPrivateKey(&n.Config.Identity, n.Identity)
	if err != nil {
		return err
	}

	n.PrivateKey = sk
	n.Peerstore.AddPrivKey(n.Identity, n.PrivateKey)
	return nil
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

// isolates the complex initialization steps
func constructPeerHost(ctx context.Context, ctxg ctxgroup.ContextGroup, cfg *config.Config, id peer.ID, ps peer.Peerstore) (p2phost.Host, error) {
	listenAddrs, err := listenAddresses(cfg)
	// make sure we dont error out if our config includes some addresses we cant use.
	filteredAddrs := swarm.FilterAddrs(listenAddrs)
	if err != nil {
		return nil, debugerror.Wrap(err)
	}
	network, err := swarm.NewNetwork(ctx, filteredAddrs, id, ps)
	if err != nil {
		return nil, debugerror.Wrap(err)
	}
	ctxg.AddChildGroup(network.CtxGroup())

	peerhost := p2pbhost.New(network)
	// explicitly set these as our listen addrs.
	// (why not do it inside inet.NewNetwork? because this way we can
	// listen on addresses without necessarily advertising those publicly.)
	addrs, err := peerhost.Network().InterfaceListenAddresses()
	if err != nil {
		return nil, debugerror.Wrap(err)
	}
	ps.AddAddresses(id, addrs)
	return peerhost, nil
}

func constructDHTRouting(ctx context.Context, ctxg ctxgroup.ContextGroup, host p2phost.Host, ds datastore.ThreadSafeDatastore) (*dht.IpfsDHT, error) {
	dhtRouting := dht.NewDHT(ctx, host, ds)
	dhtRouting.Validators[IpnsValidatorTag] = namesys.ValidateIpnsRecord
	ctxg.AddChildGroup(dhtRouting)
	return dhtRouting, nil
}
