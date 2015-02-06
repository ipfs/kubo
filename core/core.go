package core

import (
	"fmt"
	"io"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"

	diag "github.com/jbenet/go-ipfs/diagnostics"
	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	p2phost "github.com/jbenet/go-ipfs/p2p/host"
	p2pbhost "github.com/jbenet/go-ipfs/p2p/host/basic"
	swarm "github.com/jbenet/go-ipfs/p2p/net/swarm"
	addrutil "github.com/jbenet/go-ipfs/p2p/net/swarm/addr"
	peer "github.com/jbenet/go-ipfs/p2p/peer"

	routing "github.com/jbenet/go-ipfs/routing"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	offroute "github.com/jbenet/go-ipfs/routing/offline"

	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	rp "github.com/jbenet/go-ipfs/exchange/reprovide"

	mount "github.com/jbenet/go-ipfs/fuse/mount"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	namesys "github.com/jbenet/go-ipfs/namesys"
	path "github.com/jbenet/go-ipfs/path"
	config "github.com/jbenet/go-ipfs/repo/config"
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

// Mounts defines what the node's mount state is. This should
// perhaps be moved to the daemon or mount. It's here because
// it needs to be accessible across daemon requests.
type Mounts struct {
	Ipfs mount.Mount
	Ipns mount.Mount
}

func (n *IpfsNode) startOnlineServices(ctx context.Context, maybeRouter routing.IpfsRouting, nso NetworkSetupOption) error {

	if n.PeerHost != nil { // already online.
		return debugerror.New("node already online")
	}

	// load private key
	if err := n.LoadPrivateKey(); err != nil {
		return err
	}

	// Construct peer host from option
	peerhost, err := nso(ctx, n.Identity, n.Peerstore)
	if err != nil {
		return debugerror.Wrap(err)
	}
	n.PeerHost = peerhost

	if err := n.startOnlineServicesWithHost(ctx, maybeRouter); err != nil {
		return err
	}

	// Ok, now we're ready to listen.
	if err := startListening(ctx, n.PeerHost, n.Repo.Config()); err != nil {
		return debugerror.Wrap(err)
	}

	n.Reprovider = rp.NewReprovider(n.Routing, n.Blockstore)
	go n.Reprovider.ProvideEvery(ctx, kReprovideFrequency)

	return n.Bootstrap(DefaultBootstrapConfig)
}

// startOnlineServicesWithHost  is the set of services which need to be
// initialized with the host and _before_ we start listening.
func (n *IpfsNode) startOnlineServicesWithHost(ctx context.Context, maybeRouter routing.IpfsRouting) error {
	// setup diagnostics service
	n.Diagnostics = diag.NewDiagnostics(n.Identity, n.PeerHost)

	// setup routing service
	if maybeRouter != nil {
		n.Routing = maybeRouter
	} else {
		dhtRouting, err := constructDHTRouting(ctx, n.PeerHost, n.Repo.Datastore())
		if err != nil {
			return debugerror.Wrap(err)
		}
		n.Routing = dhtRouting
	}

	// setup exchange service
	const alwaysSendToPeer = true // use YesManStrategy
	bitswapNetwork := bsnet.NewFromIpfsHost(n.PeerHost, n.Routing)
	n.Exchange = bitswap.New(ctx, n.Identity, bitswapNetwork, n.Blockstore, alwaysSendToPeer)

	// setup name system
	n.Namesys = namesys.NewNameSystem(n.Routing)
	return nil
}

// teardown closes owned children. If any errors occur, this function returns
// the first error.
func (n *IpfsNode) teardown() error {
	log.Debug("core is shutting down...")
	// owned objects are closed in this teardown to ensure that they're closed
	// regardless of which constructor was used to add them to the node.
	closers := []io.Closer{
		n.Blocks,
		n.Exchange,
		n.Repo,
	}
	addCloser := func(c io.Closer) { // use when field may be nil
		if c != nil {
			closers = append(closers, c)
		}
	}

	addCloser(n.Bootstrapper)
	if dht, ok := n.Routing.(*dht.IpfsDHT); ok {
		addCloser(dht)
	}
	addCloser(n.PeerHost)

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

func (n *IpfsNode) Resolve(fpath string) (*merkledag.Node, error) {
	return n.Resolver.ResolvePath(path.Path(fpath))
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
		return debugerror.New("identity already loaded")
	}

	cid := n.Repo.Config().Identity.PeerID
	if cid == "" {
		return debugerror.New("Identity was not set in config (was ipfs init run?)")
	}
	if len(cid) == 0 {
		return debugerror.New("No peer ID in config! (was ipfs init run?)")
	}

	n.Identity = peer.ID(b58.Decode(cid))
	return nil
}

func (n *IpfsNode) LoadPrivateKey() error {
	if n.Identity == "" || n.Peerstore == nil {
		return debugerror.New("loaded private key out of order.")
	}

	if n.PrivateKey != nil {
		return debugerror.New("private key already loaded")
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

// isolates the complex initialization steps
func constructPeerHost(ctx context.Context, id peer.ID, ps peer.Peerstore) (p2phost.Host, error) {

	// no addresses to begin with. we'll start later.
	network, err := swarm.NewNetwork(ctx, nil, id, ps)
	if err != nil {
		return nil, debugerror.Wrap(err)
	}

	host := p2pbhost.New(network, p2pbhost.NATPortMap)
	return host, nil
}

// startListening on the network addresses
func startListening(ctx context.Context, host p2phost.Host, cfg *config.Config) error {
	listenAddrs, err := listenAddresses(cfg)
	if err != nil {
		return debugerror.Wrap(err)
	}

	// make sure we error out if our config does not have addresses we can use
	log.Debugf("Config.Addresses.Swarm:%s", listenAddrs)
	filteredAddrs := addrutil.FilterUsableAddrs(listenAddrs)
	log.Debugf("Config.Addresses.Swarm:%s (filtered)", filteredAddrs)
	if len(filteredAddrs) < 1 {
		return debugerror.Errorf("addresses in config not usable: %s", listenAddrs)
	}

	// Actually start listening:
	if err := host.Network().Listen(filteredAddrs...); err != nil {
		return err
	}

	// list out our addresses
	addrs, err := host.Network().InterfaceListenAddresses()
	if err != nil {
		return debugerror.Wrap(err)
	}
	log.Infof("Swarm listening at: %s", addrs)
	return nil
}

func constructDHTRouting(ctx context.Context, host p2phost.Host, ds datastore.ThreadSafeDatastore) (*dht.IpfsDHT, error) {
	dhtRouting := dht.NewDHT(ctx, host, ds)
	dhtRouting.Validator[IpnsValidatorTag] = namesys.ValidateIpnsRecord
	return dhtRouting, nil
}
