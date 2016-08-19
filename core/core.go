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

	diag "github.com/ipfs/go-ipfs/diagnostics"
	logging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
	pstore "gx/ipfs/QmQdnfvZQuhdT93LNc5bos52wAmdr3G2p6G8teLJMEN32P/go-libp2p-peerstore"
	goprocess "gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	mamask "gx/ipfs/QmSMZwvs3n4GBikZ7hKzT17c3bk65FmyZo2JqtJ16swqCv/multiaddr-filter"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	ic "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
	discovery "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/discovery"
	p2phost "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/host"
	p2pbhost "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/host/basic"
	rhost "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/host/routed"
	metrics "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/metrics"
	swarm "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/net/swarm"
	addrutil "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/net/swarm/addr"
	ping "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/protocol/ping"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	routing "github.com/ipfs/go-ipfs/routing"
	dht "github.com/ipfs/go-ipfs/routing/dht"
	nilrouting "github.com/ipfs/go-ipfs/routing/none"
	offroute "github.com/ipfs/go-ipfs/routing/offline"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	exchange "github.com/ipfs/go-ipfs/exchange"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	rp "github.com/ipfs/go-ipfs/exchange/reprovide"
	mfs "github.com/ipfs/go-ipfs/mfs"

	mount "github.com/ipfs/go-ipfs/fuse/mount"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	namesys "github.com/ipfs/go-ipfs/namesys"
	ipnsrp "github.com/ipfs/go-ipfs/namesys/republisher"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

const IpnsValidatorTag = "ipns"
const kSizeBlockstoreWriteCache = 100
const kReprovideFrequency = time.Hour * 12
const discoveryConnTimeout = time.Second * 30

var log = logging.Logger("core")

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
	Peerstore   pstore.Peerstore       // storage for other Peer instances
	Blockstore  bstore.MultiBlockstore // the block store (lower level)
	Blocks      *bserv.BlockService    // the block service, get/add blocks.
	DAG         merkledag.DAGService   // the merkle dag service, get/add objects.
	LinkService merkledag.LinkService
	Resolver    *path.Resolver // the path resolution system
	Reporter    metrics.Reporter
	Discovery   discovery.Service
	FilesRoot   *mfs.Root

	// Online
	PeerHost     p2phost.Host        // the network host (server+client)
	Bootstrapper io.Closer           // the periodic bootstrapper
	Routing      routing.IpfsRouting // the routing system. recommend ipfs-dht
	Exchange     exchange.Interface  // the block exchange + strategy (bitswap)
	Namesys      namesys.NameSystem  // the name system, resolves paths to hashes
	Diagnostics  *diag.Diagnostics   // the diagnostics service
	Ping         *ping.PingService
	Reprovider   *rp.Reprovider // the value reprovider system
	IpnsRepub    *ipnsrp.Republisher

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
	cfg, err := n.Repo.Config()
	if err != nil {
		return err
	}
	var addrfilter []*net.IPNet
	for _, s := range cfg.Swarm.AddrFilters {
		f, err := mamask.NewMask(s)
		if err != nil {
			return fmt.Errorf("incorrectly formatted address filter in config: %s", s)
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
	if err := startListening(ctx, n.PeerHost, cfg); err != nil {
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

func (n *IpfsNode) HandlePeerFound(p pstore.PeerInfo) {
	log.Warning("trying peer info: ", p)
	ctx, cancel := context.WithTimeout(n.Context(), discoveryConnTimeout)
	defer cancel()
	if err := n.PeerHost.Connect(ctx, p); err != nil {
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

	size, err := n.getCacheSize()
	if err != nil {
		return err
	}

	// setup name system
	n.Namesys = namesys.NewNameSystem(n.Routing, n.Repo.Datastore(), size)

	// setup ipns republishing
	err = n.setupIpnsRepublisher()
	if err != nil {
		return err
	}

	return nil
}

// getCacheSize returns cache life and cache size
func (n *IpfsNode) getCacheSize() (int, error) {
	cfg, err := n.Repo.Config()
	if err != nil {
		return 0, err
	}

	cs := cfg.Ipns.ResolveCacheSize
	if cs == 0 {
		cs = 128
	}
	if cs < 0 {
		return 0, fmt.Errorf("cannot specify negative resolve cache size")
	}
	return cs, nil
}

func (n *IpfsNode) setupIpnsRepublisher() error {
	cfg, err := n.Repo.Config()
	if err != nil {
		return err
	}

	n.IpnsRepub = ipnsrp.NewRepublisher(n.Routing, n.Repo.Datastore(), n.Peerstore)
	n.IpnsRepub.AddName(n.Identity)

	if cfg.Ipns.RepublishPeriod != "" {
		d, err := time.ParseDuration(cfg.Ipns.RepublishPeriod)
		if err != nil {
			return fmt.Errorf("failure to parse config setting IPNS.RepublishPeriod: %s", err)
		}

		if !u.Debug && (d < time.Minute || d > (time.Hour*24)) {
			return fmt.Errorf("config setting IPNS.RepublishPeriod is not between 1min and 1day: %s", d)
		}

		n.IpnsRepub.Interval = d
	}

	if cfg.Ipns.RecordLifetime != "" {
		d, err := time.ParseDuration(cfg.Ipns.RepublishPeriod)
		if err != nil {
			return fmt.Errorf("failure to parse config setting IPNS.RecordLifetime: %s", err)
		}

		n.IpnsRepub.RecordLifetime = d
	}

	n.Process().Go(n.IpnsRepub.Run)

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
	if n.ctx == nil {
		n.ctx = context.TODO()
	}
	return n.ctx
}

// teardown closes owned children. If any errors occur, this function returns
// the first error.
func (n *IpfsNode) teardown() error {
	log.Debug("core is shutting down...")
	// owned objects are closed in this teardown to ensure that they're closed
	// regardless of which constructor was used to add them to the node.
	var closers []io.Closer

	// NOTE: The order that objects are added(closed) matters, if an object
	// needs to use another during its shutdown/cleanup process, it should be
	// closed before that other object

	if n.FilesRoot != nil {
		closers = append(closers, n.FilesRoot)
	}

	if n.Exchange != nil {
		closers = append(closers, n.Exchange)
	}

	if n.Mounts.Ipfs != nil && !n.Mounts.Ipfs.IsActive() {
		closers = append(closers, mount.Closer(n.Mounts.Ipfs))
	}
	if n.Mounts.Ipns != nil && !n.Mounts.Ipns.IsActive() {
		closers = append(closers, mount.Closer(n.Mounts.Ipns))
	}

	if dht, ok := n.Routing.(*dht.IpfsDHT); ok {
		closers = append(closers, dht.Process())
	}

	if n.Blocks != nil {
		closers = append(closers, n.Blocks)
	}

	if n.Bootstrapper != nil {
		closers = append(closers, n.Bootstrapper)
	}

	if n.PeerHost != nil {
		closers = append(closers, n.PeerHost)
	}

	// Repo closed last, most things need to preserve state here
	closers = append(closers, n.Repo)

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
		cfg.BootstrapPeers = func() []pstore.PeerInfo {
			ps, err := n.loadBootstrapPeers()
			if err != nil {
				log.Warning("failed to parse bootstrap peers from config")
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

	cfg, err := n.Repo.Config()
	if err != nil {
		return err
	}

	cid := cfg.Identity.PeerID
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

	cfg, err := n.Repo.Config()
	if err != nil {
		return err
	}

	sk, err := loadPrivateKey(&cfg.Identity, n.Identity)
	if err != nil {
		return err
	}

	n.PrivateKey = sk
	n.Peerstore.AddPrivKey(n.Identity, n.PrivateKey)
	n.Peerstore.AddPubKey(n.Identity, sk.GetPublic())
	return nil
}

func (n *IpfsNode) loadBootstrapPeers() ([]pstore.PeerInfo, error) {
	cfg, err := n.Repo.Config()
	if err != nil {
		return nil, err
	}

	parsed, err := cfg.BootstrapPeers()
	if err != nil {
		return nil, err
	}
	return toPeerInfos(parsed), nil
}

func (n *IpfsNode) loadFilesRoot() error {
	dsk := ds.NewKey("/local/filesroot")
	pf := func(ctx context.Context, k key.Key) error {
		return n.Repo.Datastore().Put(dsk, []byte(k))
	}

	var nd *merkledag.Node
	val, err := n.Repo.Datastore().Get(dsk)

	switch {
	case err == ds.ErrNotFound || val == nil:
		nd = uio.NewEmptyDirectory()
		_, err := n.DAG.Add(nd)
		if err != nil {
			return fmt.Errorf("failure writing to dagstore: %s", err)
		}
	case err == nil:
		k := key.Key(val.([]byte))
		nd, err = n.DAG.Get(n.Context(), k)
		if err != nil {
			return fmt.Errorf("error loading filesroot from DAG: %s", err)
		}
	default:
		return err
	}

	mr, err := mfs.NewRoot(n.Context(), n.DAG, nd, pf)
	if err != nil {
		return err
	}

	n.FilesRoot = mr
	return nil
}

// SetupOfflineRouting loads the local nodes private key and
// uses it to instantiate a routing system in offline mode.
// This is primarily used for offline ipns modifications.
func (n *IpfsNode) SetupOfflineRouting() error {
	if n.Routing != nil {
		// Routing was already set up
		return nil
	}
	err := n.LoadPrivateKey()
	if err != nil {
		return err
	}

	n.Routing = offroute.NewOfflineRouter(n.Repo.Datastore(), n.PrivateKey)

	size, err := n.getCacheSize()
	if err != nil {
		return err
	}

	n.Namesys = namesys.NewNameSystem(n.Routing, n.Repo.Datastore(), size)

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

type HostOption func(ctx context.Context, id peer.ID, ps pstore.Peerstore, bwr metrics.Reporter, fs []*net.IPNet) (p2phost.Host, error)

var DefaultHostOption HostOption = constructPeerHost

// isolates the complex initialization steps
func constructPeerHost(ctx context.Context, id peer.ID, ps pstore.Peerstore, bwr metrics.Reporter, fs []*net.IPNet) (p2phost.Host, error) {

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
	dhtRouting.Selector[IpnsValidatorTag] = namesys.IpnsSelectorFunc
	return dhtRouting, nil
}

type RoutingOption func(context.Context, p2phost.Host, repo.Datastore) (routing.IpfsRouting, error)

type DiscoveryOption func(p2phost.Host) (discovery.Service, error)

var DHTOption RoutingOption = constructDHTRouting
var NilRouterOption RoutingOption = nilrouting.ConstructNilRouting
