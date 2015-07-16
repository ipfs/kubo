/*
Package core implements the IpfsNode object and related methods.

Packages underneath core/ provide a (relatively) stable, low-level API
to carry out most IPFS-related tasks.  For more details on the other
interfaces and how core/... fits into the bigger IPFS picture, see:

  $ godoc github.com/ipfs/go-ipfs
*/
package core

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	b58 "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	goprocess "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	goprocessctx "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/context"
	mamask "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/whyrusleeping/multiaddr-filter"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	diag "github.com/ipfs/go-ipfs/diagnostics"
	metrics "github.com/ipfs/go-ipfs/metrics"
	ic "github.com/ipfs/go-ipfs/p2p/crypto"
	discovery "github.com/ipfs/go-ipfs/p2p/discovery"
	p2phost "github.com/ipfs/go-ipfs/p2p/host"
	p2pbhost "github.com/ipfs/go-ipfs/p2p/host/basic"
	rhost "github.com/ipfs/go-ipfs/p2p/host/routed"
	swarm "github.com/ipfs/go-ipfs/p2p/net/swarm"
	addrutil "github.com/ipfs/go-ipfs/p2p/net/swarm/addr"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	ping "github.com/ipfs/go-ipfs/p2p/protocol/ping"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"

	routing "github.com/ipfs/go-ipfs/routing"
	dht "github.com/ipfs/go-ipfs/routing/dht"
	kb "github.com/ipfs/go-ipfs/routing/kbucket"
	offroute "github.com/ipfs/go-ipfs/routing/offline"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	exchange "github.com/ipfs/go-ipfs/exchange"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	rp "github.com/ipfs/go-ipfs/exchange/reprovide"

	mount "github.com/ipfs/go-ipfs/fuse/mount"
	ipnsfs "github.com/ipfs/go-ipfs/ipnsfs"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	namesys "github.com/ipfs/go-ipfs/namesys"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
)

const IpnsValidatorTag = "ipns"
const kSizeBlockstoreWriteCache = 100
const kReprovideFrequency = time.Hour * 12

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

	Repo repo.Repo

	// Local node
	Pinning    pin.Pinner // the pinning manager
	Mounts     Mounts     // current mount state, if any.
	PrivateKey ic.PrivKey // the local node's private Key

	// Services
	Peerstore  peer.Peerstore       // storage for other Peer instances
	Blockstore bstore.GCBlockstore  // the block store (lower level)
	Blocks     *bserv.BlockService  // the block service, get/add blocks.
	DAG        merkledag.DAGService // the merkle dag service, get/add objects.
	Resolver   *path.Resolver       // the path resolution system
	Reporter   metrics.Reporter
	Discovery  discovery.Service

	// Online
	PeerHost     p2phost.Host        // the network host (server+client)
	Bootstrapper io.Closer           // the periodic bootstrapper
	Routing      routing.IpfsRouting // the routing system. recommend ipfs-dht
	Exchange     exchange.Interface  // the block exchange + strategy (bitswap)
	Namesys      namesys.NameSystem  // the name system, resolves paths to hashes
	Diagnostics  *diag.Diagnostics   // the diagnostics service
	Ping         *ping.PingService
	Reprovider   *rp.Reprovider // the value reprovider system

	IpnsFs *ipnsfs.Filesystem

	proc goprocess.Process
	ctx  context.Context

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

	node.proc = goprocessctx.WithContextAndTeardown(ctx, node.teardown)
	node.ctx = ctx

	success := false // flip to true after all sub-system inits succeed
	defer func() {
		if !success {
			node.proc.Close()
		}
	}()

	// Need to make sure it's perfectly clear 1) which variables are expected
	// to be initialized at this point, and 2) which variables will be
	// initialized after this point.

	node.Blocks, err = bserv.New(node.Blockstore, node.Exchange)
	if err != nil {
		return nil, err
	}
	if node.Peerstore == nil {
		node.Peerstore = peer.NewPeerstore()
	}
	node.DAG = merkledag.NewDAGService(node.Blocks)
	node.Pinning, err = pin.LoadPinner(node.Repo.Datastore(), node.DAG)
	if err != nil {
		node.Pinning = pin.NewPinner(node.Repo.Datastore(), node.DAG)
	}
	node.Resolver = &path.Resolver{DAG: node.DAG}

	// Setup the mutable ipns filesystem structure
	if node.OnlineMode() {
		fs, err := ipnsfs.NewFilesystem(ctx, node.DAG, node.Namesys, node.Pinning, node.PrivateKey)
		if err != nil && err != kb.ErrLookupFailure {
			return nil, err
		}
		node.IpnsFs = fs
	}

	success = true
	return node, nil
}

func Offline(r repo.Repo) ConfigOption {
	return Standard(r, false)
}

func OnlineWithOptions(r repo.Repo, router RoutingOption, ho HostOption) ConfigOption {
	return standardWithRouting(r, true, router, ho)
}

func Online(r repo.Repo) ConfigOption {
	return Standard(r, true)
}

// DEPRECATED: use Online, Offline functions
func Standard(r repo.Repo, online bool) ConfigOption {
	return standardWithRouting(r, online, DHTOption, DefaultHostOption)
}

// TODO refactor so maybeRouter isn't special-cased in this way
func standardWithRouting(r repo.Repo, online bool, routingOption RoutingOption, hostOption HostOption) ConfigOption {
	return func(ctx context.Context) (n *IpfsNode, err error) {
		// FIXME perform node construction in the main constructor so it isn't
		// necessary to perform this teardown in this scope.
		success := false
		defer func() {
			if !success && n != nil {
				n.teardown()
			}
		}()

		// TODO move as much of node initialization as possible into
		// NewIPFSNode. The larger these config options are, the harder it is
		// to test all node construction code paths.

		if r == nil {
			return nil, fmt.Errorf("repo required")
		}
		n = &IpfsNode{
			mode: func() mode {
				if online {
					return onlineMode
				}
				return offlineMode
			}(),
			Repo: r,
		}

		// setup Peerstore
		n.Peerstore = peer.NewPeerstore()

		// setup local peer ID (private key is loaded in online setup)
		if err := n.loadID(); err != nil {
			return nil, err
		}

		n.Blockstore, err = bstore.WriteCached(bstore.NewBlockstore(n.Repo.Datastore()), kSizeBlockstoreWriteCache)
		if err != nil {
			return nil, err
		}

		if online {
			do := setupDiscoveryOption(n.Repo.Config().Discovery)
			if err := n.startOnlineServices(ctx, routingOption, hostOption, do); err != nil {
				return nil, err
			}
		} else {
			n.Exchange = offline.Exchange(n.Blockstore)
		}

		success = true
		return n, nil
	}
}

func (n *IpfsNode) startOnlineServices(ctx context.Context, routingOption RoutingOption, hostOption HostOption, do DiscoveryOption) error {

	if n.PeerHost != nil { // already online.
		return errors.New("node already online")
	}

	// load private key
	if err := n.LoadPrivateKey(); err != nil {
		return err
	}

	// Set reporter
	n.Reporter = metrics.NewBandwidthCounter()

	// get undialable addrs from config
	cfg := n.Repo.Config()
	var addrfilter []*net.IPNet
	for _, s := range cfg.Swarm.AddrFilters {
		f, err := mamask.NewMask(s)
		if err != nil {
			return fmt.Errorf("incorrectly formatter address filter in config: %s", s)
		}
		addrfilter = append(addrfilter, f)
	}

	peerhost, err := hostOption(ctx, n.Identity, n.Peerstore, n.Reporter, addrfilter)
	if err != nil {
		return err
	}

	if err := n.startOnlineServicesWithHost(ctx, peerhost, routingOption); err != nil {
		return err
	}

	// Ok, now we're ready to listen.
	if err := startListening(ctx, n.PeerHost, n.Repo.Config()); err != nil {
		return err
	}

	n.Reprovider = rp.NewReprovider(n.Routing, n.Blockstore)
	go n.Reprovider.ProvideEvery(ctx, kReprovideFrequency)

	// setup local discovery
	if do != nil {
		service, err := do(n.PeerHost)
		if err != nil {
			log.Error("mdns error: ", err)
		} else {
			service.RegisterNotifee(n)
			n.Discovery = service
		}
	}

	return n.Bootstrap(DefaultBootstrapConfig)
}

func setupDiscoveryOption(d config.Discovery) DiscoveryOption {
	if d.MDNS.Enabled {
		return func(h p2phost.Host) (discovery.Service, error) {
			if d.MDNS.Interval == 0 {
				d.MDNS.Interval = 5
			}
			return discovery.NewMdnsService(h, time.Duration(d.MDNS.Interval)*time.Second)
		}
	}
	return nil
}

func (n *IpfsNode) HandlePeerFound(p peer.PeerInfo) {
	log.Warning("trying peer info: ", p)
	ctx, _ := context.WithTimeout(context.TODO(), time.Second*10)
	err := n.PeerHost.Connect(ctx, p)
	if err != nil {
		log.Warning("Failed to connect to peer found by discovery: ", err)
	}
}

// startOnlineServicesWithHost  is the set of services which need to be
// initialized with the host and _before_ we start listening.
func (n *IpfsNode) startOnlineServicesWithHost(ctx context.Context, host p2phost.Host, routingOption RoutingOption) error {
	// setup diagnostics service
	n.Diagnostics = diag.NewDiagnostics(n.Identity, host)
	n.Ping = ping.NewPingService(host)

	// setup routing service
	r, err := routingOption(ctx, host, n.Repo.Datastore())
	if err != nil {
		return err
	}
	n.Routing = r

	// Wrap standard peer host with routing system to allow unknown peer lookups
	n.PeerHost = rhost.Wrap(host, n.Routing)

	// setup exchange service
	const alwaysSendToPeer = true // use YesManStrategy
	bitswapNetwork := bsnet.NewFromIpfsHost(n.PeerHost, n.Routing)
	n.Exchange = bitswap.New(ctx, n.Identity, bitswapNetwork, n.Blockstore, alwaysSendToPeer)

	// setup name system
	n.Namesys = namesys.NewNameSystem(n.Routing)

	return nil
}

// Process returns the Process object
func (n *IpfsNode) Process() goprocess.Process {
	return n.proc
}

// Close calls Close() on the Process object
func (n *IpfsNode) Close() error {
	return n.proc.Close()
}

// Context returns the IpfsNode context
func (n *IpfsNode) Context() context.Context {
	return n.ctx
}

// teardown closes owned children. If any errors occur, this function returns
// the first error.
func (n *IpfsNode) teardown() error {
	log.Debug("core is shutting down...")
	// owned objects are closed in this teardown to ensure that they're closed
	// regardless of which constructor was used to add them to the node.
	closers := []io.Closer{
		n.Exchange,
		n.Repo,
	}

	// Filesystem needs to be closed before network, dht, and blockservice
	// so it can use them as its shutting down
	if n.IpnsFs != nil {
		closers = append(closers, n.IpnsFs)
	}

	if n.Blocks != nil {
		closers = append(closers, n.Blocks)
	}

	if n.Bootstrapper != nil {
		closers = append(closers, n.Bootstrapper)
	}

	if dht, ok := n.Routing.(*dht.IpfsDHT); ok {
		closers = append(closers, dht.Process())
	}

	if n.PeerHost != nil {
		closers = append(closers, n.PeerHost)
	}

	var errs []error
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
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

func (n *IpfsNode) Bootstrap(cfg BootstrapConfig) error {

	// TODO what should return value be when in offlineMode?
	if n.Routing == nil {
		return nil
	}

	if n.Bootstrapper != nil {
		n.Bootstrapper.Close() // stop previous bootstrap process.
	}

	// if the caller did not specify a bootstrap peer function, get the
	// freshest bootstrap peers from config. this responds to live changes.
	if cfg.BootstrapPeers == nil {
		cfg.BootstrapPeers = func() []peer.PeerInfo {
			ps, err := n.loadBootstrapPeers()
			if err != nil {
				log.Warningf("failed to parse bootstrap peers from config: %s", n.Repo.Config().Bootstrap)
				return nil
			}
			return ps
		}
	}

	var err error
	n.Bootstrapper, err = Bootstrap(n, cfg)
	return err
}

func (n *IpfsNode) loadID() error {
	if n.Identity != "" {
		return errors.New("identity already loaded")
	}

	cid := n.Repo.Config().Identity.PeerID
	if cid == "" {
		return errors.New("Identity was not set in config (was ipfs init run?)")
	}
	if len(cid) == 0 {
		return errors.New("No peer ID in config! (was ipfs init run?)")
	}

	n.Identity = peer.ID(b58.Decode(cid))
	return nil
}

func (n *IpfsNode) LoadPrivateKey() error {
	if n.Identity == "" || n.Peerstore == nil {
		return errors.New("loaded private key out of order.")
	}

	if n.PrivateKey != nil {
		return errors.New("private key already loaded")
	}

	sk, err := loadPrivateKey(&n.Repo.Config().Identity, n.Identity)
	if err != nil {
		return err
	}

	n.PrivateKey = sk
	n.Peerstore.AddPrivKey(n.Identity, n.PrivateKey)
	n.Peerstore.AddPubKey(n.Identity, sk.GetPublic())
	return nil
}

func (n *IpfsNode) loadBootstrapPeers() ([]peer.PeerInfo, error) {
	parsed, err := n.Repo.Config().BootstrapPeers()
	if err != nil {
		return nil, err
	}
	return toPeerInfos(parsed), nil
}

// SetupOfflineRouting loads the local nodes private key and
// uses it to instantiate a routing system in offline mode.
// This is primarily used for offline ipns modifications.
func (n *IpfsNode) SetupOfflineRouting() error {
	err := n.LoadPrivateKey()
	if err != nil {
		return err
	}

	n.Routing = offroute.NewOfflineRouter(n.Repo.Datastore(), n.PrivateKey)

	n.Namesys = namesys.NewNameSystem(n.Routing)

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
	var listen []ma.Multiaddr
	for _, addr := range cfg.Addresses.Swarm {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("Failure to parse config.Addresses.Swarm: %s", cfg.Addresses.Swarm)
		}
		listen = append(listen, maddr)
	}

	return listen, nil
}

type HostOption func(ctx context.Context, id peer.ID, ps peer.Peerstore, bwr metrics.Reporter, fs []*net.IPNet) (p2phost.Host, error)

var DefaultHostOption HostOption = constructPeerHost

// isolates the complex initialization steps
func constructPeerHost(ctx context.Context, id peer.ID, ps peer.Peerstore, bwr metrics.Reporter, fs []*net.IPNet) (p2phost.Host, error) {

	// no addresses to begin with. we'll start later.
	network, err := swarm.NewNetwork(ctx, nil, id, ps, bwr)
	if err != nil {
		return nil, err
	}

	for _, f := range fs {
		network.Swarm().Filters.AddDialFilter(f)
	}

	host := p2pbhost.New(network, p2pbhost.NATPortMap, bwr)

	return host, nil
}

// startListening on the network addresses
func startListening(ctx context.Context, host p2phost.Host, cfg *config.Config) error {
	listenAddrs, err := listenAddresses(cfg)
	if err != nil {
		return err
	}

	// make sure we error out if our config does not have addresses we can use
	log.Debugf("Config.Addresses.Swarm:%s", listenAddrs)
	filteredAddrs := addrutil.FilterUsableAddrs(listenAddrs)
	log.Debugf("Config.Addresses.Swarm:%s (filtered)", filteredAddrs)
	if len(filteredAddrs) < 1 {
		return fmt.Errorf("addresses in config not usable: %s", listenAddrs)
	}

	// Actually start listening:
	if err := host.Network().Listen(filteredAddrs...); err != nil {
		return err
	}

	// list out our addresses
	addrs, err := host.Network().InterfaceListenAddresses()
	if err != nil {
		return err
	}
	log.Infof("Swarm listening at: %s", addrs)
	return nil
}

func constructDHTRouting(ctx context.Context, host p2phost.Host, dstore repo.Datastore) (routing.IpfsRouting, error) {
	dhtRouting := dht.NewDHT(ctx, host, dstore)
	dhtRouting.Validator[IpnsValidatorTag] = namesys.IpnsRecordValidator
	return dhtRouting, nil
}

type RoutingOption func(context.Context, p2phost.Host, repo.Datastore) (routing.IpfsRouting, error)

type DiscoveryOption func(p2phost.Host) (discovery.Service, error)

var DHTOption RoutingOption = constructDHTRouting
