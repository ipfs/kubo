package core

import (
	"encoding/base64"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	config "github.com/jbenet/go-ipfs/config"
	diag "github.com/jbenet/go-ipfs/diagnostics"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	"github.com/jbenet/go-ipfs/exchange/offline"
	mount "github.com/jbenet/go-ipfs/fuse/mount"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	namesys "github.com/jbenet/go-ipfs/namesys"
	inet "github.com/jbenet/go-ipfs/net"
	handshake "github.com/jbenet/go-ipfs/net/handshake"
	path "github.com/jbenet/go-ipfs/path"
	peer "github.com/jbenet/go-ipfs/peer"
	pin "github.com/jbenet/go-ipfs/pin"
	routing "github.com/jbenet/go-ipfs/routing"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
)

const IpnsValidatorTag = "ipns"
const kSizeBlockstoreWriteCache = 100

var log = eventlog.Logger("core")

// IpfsNode is IPFS Core module. It represents an IPFS instance.
type IpfsNode struct {

	// Self
	Config     *config.Config // the node's configuration
	Identity   peer.Peer      // the local node's identity
	onlineMode bool           // alternatively, offline

	// Local node
	Datastore ds2.ThreadSafeDatastoreCloser // the local datastore
	Pinning   pin.Pinner                    // the pinning manager
	Mounts    Mounts                        // current mount state, if any.

	// Services
	Peerstore   peer.Peerstore       // storage for other Peer instances
	Network     inet.Network         // the network message stream
	Routing     routing.IpfsRouting  // the routing system. recommend ipfs-dht
	Exchange    exchange.Interface   // the block exchange + strategy (bitswap)
	Blocks      *bserv.BlockService  // the block service, get/add blocks.
	DAG         merkledag.DAGService // the merkle dag service, get/add objects.
	Resolver    *path.Resolver       // the path resolution system
	Namesys     namesys.NameSystem   // the name system, resolves paths to hashes
	Diagnostics *diag.Diagnostics    // the diagnostics service

	ctxgroup.ContextGroup
}

// Mounts defines what the node's mount state is. This should
// perhaps be moved to the daemon or mount. It's here because
// it needs to be accessible across daemon requests.
type Mounts struct {
	Ipfs mount.Mount
	Ipns mount.Mount
}

// NewIpfsNode constructs a new IpfsNode based on the given config.
func NewIpfsNode(ctx context.Context, cfg *config.Config, online bool) (n *IpfsNode, err error) {
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
		onlineMode: online,
		Config:     cfg,
	}
	n.ContextGroup = ctxgroup.WithContextAndTeardown(ctx, n.teardown)
	ctx = n.ContextGroup.Context()

	// setup datastore.
	if n.Datastore, err = makeDatastore(cfg.Datastore); err != nil {
		return nil, debugerror.Wrap(err)
	}

	// setup peerstore + local peer identity
	n.Peerstore = peer.NewPeerstore()
	n.Identity, err = initIdentity(&n.Config.Identity, n.Peerstore, online)
	if err != nil {
		return nil, debugerror.Wrap(err)
	}

	blockstore, err := bstore.WriteCached(bstore.NewBlockstore(n.Datastore), kSizeBlockstoreWriteCache)
	n.Exchange = offline.Exchange(blockstore)

	// setup online services
	if online {

		// setup the network
		listenAddrs, err := listenAddresses(cfg)
		if err != nil {
			return nil, debugerror.Wrap(err)
		}

		n.Network, err = inet.NewNetwork(ctx, listenAddrs, n.Identity, n.Peerstore)
		if err != nil {
			return nil, debugerror.Wrap(err)
		}
		n.AddChildGroup(n.Network.CtxGroup())

		// setup diagnostics service
		n.Diagnostics = diag.NewDiagnostics(n.Identity, n.Network)

		// setup routing service
		dhtRouting := dht.NewDHT(ctx, n.Identity, n.Peerstore, n.Network, n.Datastore)
		dhtRouting.Validators[IpnsValidatorTag] = namesys.ValidateIpnsRecord

		// TODO(brian): perform this inside NewDHT factory method
		n.Routing = dhtRouting
		n.AddChildGroup(dhtRouting)

		// setup exchange service
		const alwaysSendToPeer = true // use YesManStrategy
		bitswapNetwork := bsnet.NewFromIpfsNetwork(n.Network)

		n.Exchange = bitswap.New(ctx, n.Identity, bitswapNetwork, n.Routing, blockstore, alwaysSendToPeer)

		// TODO consider moving connection supervision into the Network. We've
		// discussed improvements to this Node constructor. One improvement
		// would be to make the node configurable, allowing clients to inject
		// an Exchange, Network, or Routing component and have the constructor
		// manage the wiring. In that scenario, this dangling function is a bit
		// awkward.
		go superviseConnections(ctx, n.Network, dhtRouting, n.Peerstore, n.Config.Bootstrap)
	}

	// TODO(brian): when offline instantiate the BlockService with a bitswap
	// session that simply doesn't return blocks
	n.Blocks, err = bserv.New(blockstore, n.Exchange)
	if err != nil {
		return nil, debugerror.Wrap(err)
	}

	n.DAG = merkledag.NewDAGService(n.Blocks)
	n.Namesys = namesys.NewNameSystem(n.Routing)
	n.Pinning, err = pin.LoadPinner(n.Datastore, n.DAG)
	if err != nil {
		n.Pinning = pin.NewPinner(n.Datastore, n.DAG)
	}
	n.Resolver = &path.Resolver{DAG: n.DAG}

	success = true
	return n, nil
}

func (n *IpfsNode) teardown() error {
	if err := n.Datastore.Close(); err != nil {
		return err
	}
	return nil
}

func (n *IpfsNode) OnlineMode() bool {
	return n.onlineMode
}

func initIdentity(cfg *config.Identity, peers peer.Peerstore, online bool) (peer.Peer, error) {
	if cfg.PeerID == "" {
		return nil, debugerror.New("Identity was not set in config (was ipfs init run?)")
	}

	if len(cfg.PeerID) == 0 {
		return nil, debugerror.New("No peer ID in config! (was ipfs init run?)")
	}

	// get peer from peerstore (so it is constructed there)
	id := peer.ID(b58.Decode(cfg.PeerID))
	self, err := peers.FindOrCreate(id)
	if err != nil {
		return nil, err
	}
	self.SetType(peer.Local)
	self, err = peers.Add(self)
	if err != nil {
		return nil, err
	}

	self.SetVersions(handshake.ClientVersion, handshake.IpfsVersion.String())

	// when not online, don't need to parse private keys (yet)
	if online {
		skb, err := base64.StdEncoding.DecodeString(cfg.PrivKey)
		if err != nil {
			return nil, err
		}

		if err := self.LoadAndVerifyKeyPair(skb); err != nil {
			return nil, err
		}
	}

	return self, nil
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
