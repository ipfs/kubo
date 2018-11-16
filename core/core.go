/*
Package core implements the IpfsNode object and related methods.

Packages underneath core/ provide a (relatively) stable, low-level API
to carry out most IPFS-related tasks.  For more details on the other
interfaces and how core/... fits into the bigger IPFS picture, see:

  $ godoc github.com/ipfs/go-ipfs
*/
package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	version "github.com/ipfs/go-ipfs"
	rp "github.com/ipfs/go-ipfs/exchange/reprovide"
	filestore "github.com/ipfs/go-ipfs/filestore"
	mount "github.com/ipfs/go-ipfs/fuse/mount"
	namesys "github.com/ipfs/go-ipfs/namesys"
	ipnsrp "github.com/ipfs/go-ipfs/namesys/republisher"
	p2p "github.com/ipfs/go-ipfs/p2p"
	pin "github.com/ipfs/go-ipfs/pin"
	repo "github.com/ipfs/go-ipfs/repo"

	ic "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	u "gx/ipfs/QmNohiVssaPw3KVLZik59DBVGTSm2dGvYT9eoXt5DQ36Yz/go-ipfs-util"
	exchange "gx/ipfs/QmP2g3VxmC7g7fyRJDj1VJ72KHZbJ9UW24YjSWEj1XTb4H/go-ipfs-exchange-interface"
	pstore "gx/ipfs/QmQAGG1zxfePqj2t7bLxyN8AFccZ889DDR9Gn8kVLDrGZo/go-libp2p-peerstore"
	mafilter "gx/ipfs/QmQJRvWaYAvU3Mdtk33ADXr9JAZwKMBYBGPkRQBDvyj2nn/go-maddr-filter"
	ifconnmgr "gx/ipfs/QmQSucBpqUVQ5Q1stDmm2Bon4Tq4KNhNXuVmLMraARoUoh/go-libp2p-interface-connmgr"
	dht "gx/ipfs/QmQsw6Nq2A345PqChdtbWVoYbSno7uqRDHwYmYpbPHmZNc/go-libp2p-kad-dht"
	dhtopts "gx/ipfs/QmQsw6Nq2A345PqChdtbWVoYbSno7uqRDHwYmYpbPHmZNc/go-libp2p-kad-dht/opts"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	goprocess "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	mamask "gx/ipfs/QmSMZwvs3n4GBikZ7hKzT17c3bk65FmyZo2JqtJ16swqCv/multiaddr-filter"
	record "gx/ipfs/QmSoeYGNm8v8jAF49hX7UwHwkXjoeobSrn9sya5NPPsxXP/go-libp2p-record"
	rhelpers "gx/ipfs/QmTCSuGSboWbTtyZpTgzoR6tytKy48x5XUpFuXTCM69zuE/go-libp2p-routing-helpers"
	bitswap "gx/ipfs/QmTxeg52XprLb5j3yaP1nAP3K7sGNkG1pjrHEwBMGFfcf6/go-bitswap"
	bsnet "gx/ipfs/QmTxeg52XprLb5j3yaP1nAP3K7sGNkG1pjrHEwBMGFfcf6/go-bitswap/network"
	ft "gx/ipfs/QmUnHNqhSB1JgzVCxL1Kz3yb4bdyB4q1Z9AD5AUBVmt3fZ/go-unixfs"
	psrouter "gx/ipfs/QmV1Z5sZeGL1yy4aKheYgByb4vpaiHrDiwewcH3Sq4YVrZ/go-libp2p-pubsub-router"
	connmgr "gx/ipfs/QmV4Z9ufnVy7tMFHfBnoxbbAxDwFfKNKWtZAbY4CHCp79W/go-libp2p-connmgr"
	mfs "gx/ipfs/QmV8mXUh1M9qztax7vVdL1Apuz4c1eJZC5YactGxaJfWom/go-mfs"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	resolver "gx/ipfs/QmVi2uUygezqaMTqs3Yzt5FcZFHJoYD4B7jQ2BELjj7ZuY/go-path/resolver"
	libp2p "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p"
	discovery "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p/p2p/discovery"
	p2pbhost "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p/p2p/host/basic"
	rhost "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p/p2p/host/routed"
	identify "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p/p2p/protocol/identify"
	quic "gx/ipfs/QmWY1pHdRP1rA2ifUuCu1ZwFJ8ZzpSEcgXsu9haH21AYKd/go-libp2p-quic-transport"
	circuit "gx/ipfs/QmWxCQ66YzAb24Py5D3qeDJcVTF4aW9AeAmppQ7FhQZxgZ/go-libp2p-circuit"
	pnet "gx/ipfs/QmY4Q5JC4vxLEi8EpVxJM4rcRryEVtH1zRKVTAm6BKV1pg/go-libp2p-pnet"
	smux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	routing "gx/ipfs/QmZBH87CAPFHcc7cYmBqeSQ98zQ3SX9KUxiYgzPmLWNVKz/go-libp2p-routing"
	mplex "gx/ipfs/QmZsejKNkeFSQe5TcmYXJ8iq6qPL1FpsP4eAA8j7RfE7xg/go-smux-multiplex"
	p2phost "gx/ipfs/QmahxMNoNuSsgQefo9rkpcfRFmQrMN6Q99aztKXf63K7YJ/go-libp2p-host"
	config "gx/ipfs/QmbK4EmM2Xx5fmbqK38TGP3PpY66r3tkXLZTcc7dF9mFwM/go-ipfs-config"
	pubsub "gx/ipfs/Qmc3BYVGtLs8y3p4uVpARWyo3Xk2oCBFF1AhYUVMPWgwUK/go-libp2p-pubsub"
	merkledag "gx/ipfs/QmcGt25mrjuB2kKW2zhPbXVZNHc4yoTDQ65NA8m6auP2f1/go-merkledag"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
	peer "gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
	yamux "gx/ipfs/Qmdps3CYh5htGQSrPvzg5PHouVexLmtpbuLCqc4vuej8PC/go-smux-yamux"
	nilrouting "gx/ipfs/QmdxhyAwBrnmJFsYPK6tyHh4Yy3gK8gbULErX1dRnpUMqu/go-ipfs-routing/none"
	offroute "gx/ipfs/QmdxhyAwBrnmJFsYPK6tyHh4Yy3gK8gbULErX1dRnpUMqu/go-ipfs-routing/offline"
	ds "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
	metrics "gx/ipfs/QmfBAmuDFoPTMC232UQenPDYAzHQ48crKaXG9AfQqFuRpN/go-libp2p-metrics"
)

const IpnsValidatorTag = "ipns"

const kReprovideFrequency = time.Hour * 12
const discoveryConnTimeout = time.Second * 30

var log = logging.Logger("core")

type mode int

const (
	// zero value is not a valid mode, must be explicitly set
	localMode mode = iota
	offlineMode
	onlineMode
)

func init() {
	identify.ClientVersion = "go-ipfs/" + version.CurrentVersionNumber + "/" + version.CurrentCommit
}

// IpfsNode is IPFS Core module. It represents an IPFS instance.
type IpfsNode struct {

	// Self
	Identity peer.ID // the local node's identity

	Repo repo.Repo

	// Local node
	Pinning         pin.Pinner // the pinning manager
	Mounts          Mounts     // current mount state, if any.
	PrivateKey      ic.PrivKey // the local node's private Key
	PNetFingerprint []byte     // fingerprint of private network

	// Services
	Peerstore       pstore.Peerstore     // storage for other Peer instances
	Blockstore      bstore.GCBlockstore  // the block store (lower level)
	Filestore       *filestore.Filestore // the filestore blockstore
	BaseBlocks      bstore.Blockstore    // the raw blockstore, no filestore wrapping
	GCLocker        bstore.GCLocker      // the locker used to protect the blockstore during gc
	Blocks          bserv.BlockService   // the block service, get/add blocks.
	DAG             ipld.DAGService      // the merkle dag service, get/add objects.
	Resolver        *resolver.Resolver   // the path resolution system
	Reporter        metrics.Reporter
	Discovery       discovery.Service
	FilesRoot       *mfs.Root
	RecordValidator record.Validator

	// Online
	PeerHost     p2phost.Host        // the network host (server+client)
	Bootstrapper io.Closer           // the periodic bootstrapper
	Routing      routing.IpfsRouting // the routing system. recommend ipfs-dht
	Exchange     exchange.Interface  // the block exchange + strategy (bitswap)
	Namesys      namesys.NameSystem  // the name system, resolves paths to hashes
	Reprovider   *rp.Reprovider      // the value reprovider system
	IpnsRepub    *ipnsrp.Republisher

	PubSub   *pubsub.PubSub
	PSRouter *psrouter.PubsubValueStore
	DHT      *dht.IpfsDHT
	P2P      *p2p.P2P

	proc goprocess.Process
	ctx  context.Context

	mode         mode
	localModeSet bool
}

// Mounts defines what the node's mount state is. This should
// perhaps be moved to the daemon or mount. It's here because
// it needs to be accessible across daemon requests.
type Mounts struct {
	Ipfs mount.Mount
	Ipns mount.Mount
}

func (n *IpfsNode) startOnlineServices(ctx context.Context, routingOption RoutingOption, hostOption HostOption, do DiscoveryOption, pubsub, ipnsps, mplex bool) error {
	if n.PeerHost != nil { // already online.
		return errors.New("node already online")
	}

	// load private key
	if err := n.LoadPrivateKey(); err != nil {
		return err
	}

	// get undialable addrs from config
	cfg, err := n.Repo.Config()
	if err != nil {
		return err
	}

	var libp2pOpts []libp2p.Option
	for _, s := range cfg.Swarm.AddrFilters {
		f, err := mamask.NewMask(s)
		if err != nil {
			return fmt.Errorf("incorrectly formatted address filter in config: %s", s)
		}
		libp2pOpts = append(libp2pOpts, libp2p.FilterAddresses(f))
	}

	if !cfg.Swarm.DisableBandwidthMetrics {
		// Set reporter
		n.Reporter = metrics.NewBandwidthCounter()
		libp2pOpts = append(libp2pOpts, libp2p.BandwidthReporter(n.Reporter))
	}

	swarmkey, err := n.Repo.SwarmKey()
	if err != nil {
		return err
	}

	if swarmkey != nil {
		protec, err := pnet.NewProtector(bytes.NewReader(swarmkey))
		if err != nil {
			return fmt.Errorf("failed to configure private network: %s", err)
		}
		n.PNetFingerprint = protec.Fingerprint()
		go func() {
			t := time.NewTicker(30 * time.Second)
			<-t.C // swallow one tick
			for {
				select {
				case <-t.C:
					if ph := n.PeerHost; ph != nil {
						if len(ph.Network().Peers()) == 0 {
							log.Warning("We are in private network and have no peers.")
							log.Warning("This might be configuration mistake.")
						}
					}
				case <-n.Process().Closing():
					t.Stop()
					return
				}
			}
		}()

		libp2pOpts = append(libp2pOpts, libp2p.PrivateNetwork(protec))
	}

	addrsFactory, err := makeAddrsFactory(cfg.Addresses)
	if err != nil {
		return err
	}
	if !cfg.Swarm.DisableRelay {
		addrsFactory = composeAddrsFactory(addrsFactory, filterRelayAddrs)
	}
	libp2pOpts = append(libp2pOpts, libp2p.AddrsFactory(addrsFactory))

	connm, err := constructConnMgr(cfg.Swarm.ConnMgr)
	if err != nil {
		return err
	}
	libp2pOpts = append(libp2pOpts, libp2p.ConnectionManager(connm))

	libp2pOpts = append(libp2pOpts, makeSmuxTransportOption(mplex))

	if !cfg.Swarm.DisableNatPortMap {
		libp2pOpts = append(libp2pOpts, libp2p.NATPortMap())
	}

	// disable the default listen addrs
	libp2pOpts = append(libp2pOpts, libp2p.NoListenAddrs)

	if cfg.Swarm.DisableRelay {
		// Enabled by default.
		libp2pOpts = append(libp2pOpts, libp2p.DisableRelay())
	} else {
		relayOpts := []circuit.RelayOpt{circuit.OptDiscovery}
		if cfg.Swarm.EnableRelayHop {
			relayOpts = append(relayOpts, circuit.OptHop)
		}
		libp2pOpts = append(libp2pOpts, libp2p.EnableRelay(relayOpts...))
	}

	// explicitly enable the default transports
	libp2pOpts = append(libp2pOpts, libp2p.DefaultTransports)

	if cfg.Experimental.QUIC {
		libp2pOpts = append(libp2pOpts, libp2p.Transport(quic.NewTransport))
	}

	peerhost, err := hostOption(ctx, n.Identity, n.Peerstore, libp2pOpts...)

	if err != nil {
		return err
	}

	if err := n.startOnlineServicesWithHost(ctx, peerhost, routingOption, pubsub, ipnsps); err != nil {
		return err
	}

	// Ok, now we're ready to listen.
	if err := startListening(n.PeerHost, cfg); err != nil {
		return err
	}

	n.P2P = p2p.NewP2P(n.Identity, n.PeerHost, n.Peerstore)

	// setup local discovery
	if do != nil {
		service, err := do(ctx, n.PeerHost)
		if err != nil {
			log.Error("mdns error: ", err)
		} else {
			service.RegisterNotifee(n)
			n.Discovery = service
		}
	}

	return n.Bootstrap(DefaultBootstrapConfig)
}

func constructConnMgr(cfg config.ConnMgr) (ifconnmgr.ConnManager, error) {
	switch cfg.Type {
	case "":
		// 'default' value is the basic connection manager
		return connmgr.NewConnManager(config.DefaultConnMgrLowWater, config.DefaultConnMgrHighWater, config.DefaultConnMgrGracePeriod), nil
	case "none":
		return nil, nil
	case "basic":
		grace, err := time.ParseDuration(cfg.GracePeriod)
		if err != nil {
			return nil, fmt.Errorf("parsing Swarm.ConnMgr.GracePeriod: %s", err)
		}

		return connmgr.NewConnManager(cfg.LowWater, cfg.HighWater, grace), nil
	default:
		return nil, fmt.Errorf("unrecognized ConnMgr.Type: %q", cfg.Type)
	}
}

func (n *IpfsNode) startLateOnlineServices(ctx context.Context) error {
	cfg, err := n.Repo.Config()
	if err != nil {
		return err
	}

	var keyProvider rp.KeyChanFunc

	switch cfg.Reprovider.Strategy {
	case "all":
		fallthrough
	case "":
		keyProvider = rp.NewBlockstoreProvider(n.Blockstore)
	case "roots":
		keyProvider = rp.NewPinnedProvider(n.Pinning, n.DAG, true)
	case "pinned":
		keyProvider = rp.NewPinnedProvider(n.Pinning, n.DAG, false)
	default:
		return fmt.Errorf("unknown reprovider strategy '%s'", cfg.Reprovider.Strategy)
	}
	n.Reprovider = rp.NewReprovider(ctx, n.Routing, keyProvider)

	reproviderInterval := kReprovideFrequency
	if cfg.Reprovider.Interval != "" {
		dur, err := time.ParseDuration(cfg.Reprovider.Interval)
		if err != nil {
			return err
		}

		reproviderInterval = dur
	}

	go n.Reprovider.Run(reproviderInterval)

	return nil
}

func makeAddrsFactory(cfg config.Addresses) (p2pbhost.AddrsFactory, error) {
	var annAddrs []ma.Multiaddr
	for _, addr := range cfg.Announce {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		annAddrs = append(annAddrs, maddr)
	}

	filters := mafilter.NewFilters()
	noAnnAddrs := map[string]bool{}
	for _, addr := range cfg.NoAnnounce {
		f, err := mamask.NewMask(addr)
		if err == nil {
			filters.AddDialFilter(f)
			continue
		}
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		noAnnAddrs[maddr.String()] = true
	}

	return func(allAddrs []ma.Multiaddr) []ma.Multiaddr {
		var addrs []ma.Multiaddr
		if len(annAddrs) > 0 {
			addrs = annAddrs
		} else {
			addrs = allAddrs
		}

		var out []ma.Multiaddr
		for _, maddr := range addrs {
			// check for exact matches
			ok, _ := noAnnAddrs[maddr.String()]
			// check for /ipcidr matches
			if !ok && !filters.AddrBlocked(maddr) {
				out = append(out, maddr)
			}
		}
		return out
	}, nil
}

func makeSmuxTransportOption(mplexExp bool) libp2p.Option {
	const yamuxID = "/yamux/1.0.0"
	const mplexID = "/mplex/6.7.0"

	ymxtpt := &yamux.Transport{
		AcceptBacklog:          512,
		ConnectionWriteTimeout: time.Second * 10,
		KeepAliveInterval:      time.Second * 30,
		EnableKeepAlive:        true,
		MaxStreamWindowSize:    uint32(1024 * 512),
		LogOutput:              ioutil.Discard,
	}

	if os.Getenv("YAMUX_DEBUG") != "" {
		ymxtpt.LogOutput = os.Stderr
	}

	muxers := map[string]smux.Transport{yamuxID: ymxtpt}
	if mplexExp {
		muxers[mplexID] = mplex.DefaultTransport
	}

	// Allow muxer preference order overriding
	order := []string{yamuxID, mplexID}
	if prefs := os.Getenv("LIBP2P_MUX_PREFS"); prefs != "" {
		order = strings.Fields(prefs)
	}

	opts := make([]libp2p.Option, 0, len(order))
	for _, id := range order {
		tpt, ok := muxers[id]
		if !ok {
			log.Warning("unknown or duplicate muxer in LIBP2P_MUX_PREFS: %s", id)
			continue
		}
		delete(muxers, id)
		opts = append(opts, libp2p.Muxer(id, tpt))
	}

	return libp2p.ChainOptions(opts...)
}

func setupDiscoveryOption(d config.Discovery) DiscoveryOption {
	if d.MDNS.Enabled {
		return func(ctx context.Context, h p2phost.Host) (discovery.Service, error) {
			if d.MDNS.Interval == 0 {
				d.MDNS.Interval = 5
			}
			return discovery.NewMdnsService(ctx, h, time.Duration(d.MDNS.Interval)*time.Second, discovery.ServiceTag)
		}
	}
	return nil
}

// HandlePeerFound attempts to connect to peer from `PeerInfo`, if it fails
// logs a warning log.
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
func (n *IpfsNode) startOnlineServicesWithHost(ctx context.Context, host p2phost.Host, routingOption RoutingOption, enablePubsub bool, enableIpnsps bool) error {
	if enablePubsub || enableIpnsps {
		cfg, err := n.Repo.Config()
		if err != nil {
			return err
		}

		var service *pubsub.PubSub

		var pubsubOptions []pubsub.Option
		if cfg.Pubsub.DisableSigning {
			pubsubOptions = append(pubsubOptions, pubsub.WithMessageSigning(false))
		}

		if cfg.Pubsub.StrictSignatureVerification {
			pubsubOptions = append(pubsubOptions, pubsub.WithStrictSignatureVerification(true))
		}

		switch cfg.Pubsub.Router {
		case "":
			fallthrough
		case "floodsub":
			service, err = pubsub.NewFloodSub(ctx, host, pubsubOptions...)

		case "gossipsub":
			service, err = pubsub.NewGossipSub(ctx, host, pubsubOptions...)

		default:
			err = fmt.Errorf("Unknown pubsub router %s", cfg.Pubsub.Router)
		}

		if err != nil {
			return err
		}
		n.PubSub = service
	}

	// setup routing service
	r, err := routingOption(ctx, host, n.Repo.Datastore(), n.RecordValidator)
	if err != nil {
		return err
	}
	n.Routing = r

	// TODO: I'm not a fan of type assertions like this but the
	// `RoutingOption` system doesn't currently provide access to the
	// IpfsNode.
	//
	// Ideally, we'd do something like:
	//
	// 1. Add some fancy method to introspect into tiered routers to extract
	//    things like the pubsub router or the DHT (complicated, messy,
	//    probably not worth it).
	// 2. Pass the IpfsNode into the RoutingOption (would also remove the
	//    PSRouter case below.
	// 3. Introduce some kind of service manager? (my personal favorite but
	//    that requires a fair amount of work).
	if dht, ok := r.(*dht.IpfsDHT); ok {
		n.DHT = dht
	}

	if enableIpnsps {
		n.PSRouter = psrouter.NewPubsubValueStore(
			ctx,
			host,
			n.Routing,
			n.PubSub,
			n.RecordValidator,
		)
		n.Routing = rhelpers.Tiered{
			Routers: []routing.IpfsRouting{
				// Always check pubsub first.
				&rhelpers.Compose{
					ValueStore: &rhelpers.LimitedValueStore{
						ValueStore: n.PSRouter,
						Namespaces: []string{"ipns"},
					},
				},
				n.Routing,
			},
			Validator: n.RecordValidator,
		}
	}

	// Wrap standard peer host with routing system to allow unknown peer lookups
	n.PeerHost = rhost.Wrap(host, n.Routing)

	// setup exchange service
	bitswapNetwork := bsnet.NewFromIpfsHost(n.PeerHost, n.Routing)
	n.Exchange = bitswap.New(ctx, bitswapNetwork, n.Blockstore)

	size, err := n.getCacheSize()
	if err != nil {
		return err
	}

	// setup name system
	n.Namesys = namesys.NewNameSystem(n.Routing, n.Repo.Datastore(), size)

	// setup ipns republishing
	return n.setupIpnsRepublisher()
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

	n.IpnsRepub = ipnsrp.NewRepublisher(n.Namesys, n.Repo.Datastore(), n.PrivateKey, n.Repo.Keystore())

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
		d, err := time.ParseDuration(cfg.Ipns.RecordLifetime)
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

	if n.DHT != nil {
		closers = append(closers, n.DHT.Process())
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

// OnlineMode returns whether or not the IpfsNode is in OnlineMode.
func (n *IpfsNode) OnlineMode() bool {
	return n.mode == onlineMode
}

// SetLocal will set the IpfsNode to local mode
func (n *IpfsNode) SetLocal(isLocal bool) {
	if isLocal {
		n.mode = localMode
	}
	n.localModeSet = true
}

// LocalMode returns whether or not the IpfsNode is in LocalMode
func (n *IpfsNode) LocalMode() bool {
	if !n.localModeSet {
		// programmer error should not happen
		panic("local mode not set")
	}
	return n.mode == localMode
}

// Bootstrap will set and call the IpfsNodes bootstrap function.
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
		return errors.New("identity was not set in config (was 'ipfs init' run?)")
	}
	if len(cid) == 0 {
		return errors.New("no peer ID in config! (was 'ipfs init' run?)")
	}

	id, err := peer.IDB58Decode(cid)
	if err != nil {
		return fmt.Errorf("peer ID invalid: %s", err)
	}

	n.Identity = id
	return nil
}

// GetKey will return a key from the Keystore with name `name`.
func (n *IpfsNode) GetKey(name string) (ic.PrivKey, error) {
	if name == "self" {
		return n.PrivateKey, nil
	} else {
		return n.Repo.Keystore().Get(name)
	}
}

func (n *IpfsNode) LoadPrivateKey() error {
	if n.Identity == "" || n.Peerstore == nil {
		return errors.New("loaded private key out of order")
	}

	if n.PrivateKey != nil {
		log.Warning("private key already loaded")
		return nil
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
	pf := func(ctx context.Context, c cid.Cid) error {
		return n.Repo.Datastore().Put(dsk, c.Bytes())
	}

	var nd *merkledag.ProtoNode
	val, err := n.Repo.Datastore().Get(dsk)

	switch {
	case err == ds.ErrNotFound || val == nil:
		nd = ft.EmptyDirNode()
		err := n.DAG.Add(n.Context(), nd)
		if err != nil {
			return fmt.Errorf("failure writing to dagstore: %s", err)
		}
	case err == nil:
		c, err := cid.Cast(val)
		if err != nil {
			return err
		}

		rnd, err := n.DAG.Get(n.Context(), c)
		if err != nil {
			return fmt.Errorf("error loading filesroot from DAG: %s", err)
		}

		pbnd, ok := rnd.(*merkledag.ProtoNode)
		if !ok {
			return merkledag.ErrNotProtobuf
		}

		nd = pbnd
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

// SetupOfflineRouting instantiates a routing system in offline mode. This is
// primarily used for offline ipns modifications.
func (n *IpfsNode) SetupOfflineRouting() error {
	if n.Routing != nil {
		// Routing was already set up
		return nil
	}

	// TODO: move this somewhere else.
	err := n.LoadPrivateKey()
	if err != nil {
		return err
	}

	n.Routing = offroute.NewOfflineRouter(n.Repo.Datastore(), n.RecordValidator)

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
			return nil, fmt.Errorf("failure to parse config.Addresses.Swarm: %s", cfg.Addresses.Swarm)
		}
		listen = append(listen, maddr)
	}

	return listen, nil
}

type ConstructPeerHostOpts struct {
	AddrsFactory      p2pbhost.AddrsFactory
	DisableNatPortMap bool
	DisableRelay      bool
	EnableRelayHop    bool
	ConnectionManager ifconnmgr.ConnManager
}

type HostOption func(ctx context.Context, id peer.ID, ps pstore.Peerstore, options ...libp2p.Option) (p2phost.Host, error)

var DefaultHostOption HostOption = constructPeerHost

// isolates the complex initialization steps
func constructPeerHost(ctx context.Context, id peer.ID, ps pstore.Peerstore, options ...libp2p.Option) (p2phost.Host, error) {
	pkey := ps.PrivKey(id)
	if pkey == nil {
		return nil, fmt.Errorf("missing private key for node ID: %s", id.Pretty())
	}
	options = append([]libp2p.Option{libp2p.Identity(pkey), libp2p.Peerstore(ps)}, options...)
	return libp2p.New(ctx, options...)
}

func filterRelayAddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	var raddrs []ma.Multiaddr
	for _, addr := range addrs {
		_, err := addr.ValueForProtocol(circuit.P_CIRCUIT)
		if err == nil {
			continue
		}
		raddrs = append(raddrs, addr)
	}
	return raddrs
}

func composeAddrsFactory(f, g p2pbhost.AddrsFactory) p2pbhost.AddrsFactory {
	return func(addrs []ma.Multiaddr) []ma.Multiaddr {
		return f(g(addrs))
	}
}

// startListening on the network addresses
func startListening(host p2phost.Host, cfg *config.Config) error {
	listenAddrs, err := listenAddresses(cfg)
	if err != nil {
		return err
	}

	// Actually start listening:
	if err := host.Network().Listen(listenAddrs...); err != nil {
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

func constructDHTRouting(ctx context.Context, host p2phost.Host, dstore ds.Batching, validator record.Validator) (routing.IpfsRouting, error) {
	return dht.New(
		ctx, host,
		dhtopts.Datastore(dstore),
		dhtopts.Validator(validator),
	)
}

func constructClientDHTRouting(ctx context.Context, host p2phost.Host, dstore ds.Batching, validator record.Validator) (routing.IpfsRouting, error) {
	return dht.New(
		ctx, host,
		dhtopts.Client(true),
		dhtopts.Datastore(dstore),
		dhtopts.Validator(validator),
	)
}

type RoutingOption func(context.Context, p2phost.Host, ds.Batching, record.Validator) (routing.IpfsRouting, error)

type DiscoveryOption func(context.Context, p2phost.Host) (discovery.Service, error)

var DHTOption RoutingOption = constructDHTRouting
var DHTClientOption RoutingOption = constructClientDHTRouting
var NilRouterOption RoutingOption = nilrouting.ConstructNilRouting
