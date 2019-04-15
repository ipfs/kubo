package node

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs-config"
	nilrouting "github.com/ipfs/go-ipfs-routing/none"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-autonat-svc"
	"github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/libp2p/go-libp2p-host"
	"github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	"github.com/libp2p/go-libp2p-metrics"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"github.com/libp2p/go-libp2p-pnet"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p-pubsub-router"
	"github.com/libp2p/go-libp2p-quic-transport"
	"github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p-routing"
	"github.com/libp2p/go-libp2p-routing-helpers"
	p2pbhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/host/routed"
	mafilter "github.com/libp2p/go-maddr-filter"
	smux "github.com/libp2p/go-stream-muxer"
	ma "github.com/multiformats/go-multiaddr"
	mplex "github.com/whyrusleeping/go-smux-multiplex"
	yamux "github.com/whyrusleeping/go-smux-yamux"
	"github.com/whyrusleeping/multiaddr-filter"
	mamask "github.com/whyrusleeping/multiaddr-filter"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/repo"
)

var log = logging.Logger("node")

type HostOption func(ctx context.Context, id peer.ID, ps peerstore.Peerstore, options ...libp2p.Option) (host.Host, error)
type RoutingOption func(context.Context, host.Host, datastore.Batching, record.Validator) (routing.IpfsRouting, error)

var DefaultHostOption HostOption = constructPeerHost

// isolates the complex initialization steps
func constructPeerHost(ctx context.Context, id peer.ID, ps peerstore.Peerstore, options ...libp2p.Option) (host.Host, error) {
	pkey := ps.PrivKey(id)
	if pkey == nil {
		return nil, fmt.Errorf("missing private key for node ID: %s", id.Pretty())
	}
	options = append([]libp2p.Option{libp2p.Identity(pkey), libp2p.Peerstore(ps)}, options...)
	return libp2p.New(ctx, options...)
}

func constructDHTRouting(ctx context.Context, host host.Host, dstore datastore.Batching, validator record.Validator) (routing.IpfsRouting, error) {
	return dht.New(
		ctx, host,
		dhtopts.Datastore(dstore),
		dhtopts.Validator(validator),
	)
}

func constructClientDHTRouting(ctx context.Context, host host.Host, dstore datastore.Batching, validator record.Validator) (routing.IpfsRouting, error) {
	return dht.New(
		ctx, host,
		dhtopts.Client(true),
		dhtopts.Datastore(dstore),
		dhtopts.Validator(validator),
	)
}

var DHTOption RoutingOption = constructDHTRouting
var DHTClientOption RoutingOption = constructClientDHTRouting
var NilRouterOption RoutingOption = nilrouting.ConstructNilRouting

func Peerstore(id peer.ID, sk crypto.PrivKey) peerstore.Peerstore {
	ps := pstoremem.NewPeerstore()

	if sk != nil {
		ps.AddPrivKey(id, sk)
		ps.AddPubKey(id, sk.GetPublic())
	}

	return ps
}

func P2PAddrFilters(cfg *config.Config) (opts Libp2pOpts, err error) {
	for _, s := range cfg.Swarm.AddrFilters {
		f, err := mask.NewMask(s)
		if err != nil {
			return opts, fmt.Errorf("incorrectly formatted address filter in config: %s", s)
		}
		opts.Opts = append(opts.Opts, libp2p.FilterAddresses(f))
	}
	return opts, nil
}

func P2PBandwidthCounter(cfg *config.Config) (opts Libp2pOpts, reporter metrics.Reporter) {
	reporter = metrics.NewBandwidthCounter()

	if !cfg.Swarm.DisableBandwidthMetrics {
		opts.Opts = append(opts.Opts, libp2p.BandwidthReporter(reporter))
	}
	return opts, reporter
}

type Libp2pOpts struct {
	fx.Out

	Opts []libp2p.Option `group:"libp2p"`
}

type PNetFingerprint []byte

func P2PPNet(repo repo.Repo) (opts Libp2pOpts, fp PNetFingerprint, err error) {
	swarmkey, err := repo.SwarmKey()
	if err != nil || swarmkey == nil {
		return opts, nil, err
	}

	protec, err := pnet.NewProtector(bytes.NewReader(swarmkey))
	if err != nil {
		return opts, nil, fmt.Errorf("failed to configure private network: %s", err)
	}
	fp = protec.Fingerprint()

	opts.Opts = append(opts.Opts, libp2p.PrivateNetwork(protec))
	return opts, fp, nil
}

func P2PPNetChecker(repo repo.Repo, ph host.Host, lc fx.Lifecycle) error {
	// TODO: better check?
	swarmkey, err := repo.SwarmKey()
	if err != nil || swarmkey == nil {
		return err
	}

	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				t := time.NewTicker(30 * time.Second)
				defer t.Stop()

				<-t.C // swallow one tick
				for {
					select {
					case <-t.C:
						if len(ph.Network().Peers()) == 0 {
							log.Warning("We are in private network and have no peers.")
							log.Warning("This might be configuration mistake.")
						}
					case <-done:
						return
					}
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			close(done)
			return nil
		},
	})
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
			ok := noAnnAddrs[maddr.String()]
			// check for /ipcidr matches
			if !ok && !filters.AddrBlocked(maddr) {
				out = append(out, maddr)
			}
		}
		return out
	}, nil
}

func P2PAddrsFactory(cfg *config.Config) (opts Libp2pOpts, err error) {
	addrsFactory, err := makeAddrsFactory(cfg.Addresses)
	if err != nil {
		return opts, err
	}
	opts.Opts = append(opts.Opts, libp2p.AddrsFactory(addrsFactory))
	return
}

func P2PConnectionManager(cfg *config.Config) (opts Libp2pOpts, err error) {
	grace := config.DefaultConnMgrGracePeriod
	low := config.DefaultConnMgrHighWater
	high := config.DefaultConnMgrHighWater

	switch cfg.Swarm.ConnMgr.Type {
	case "":
		// 'default' value is the basic connection manager
		return
	case "none":
		return opts, nil
	case "basic":
		grace, err = time.ParseDuration(cfg.Swarm.ConnMgr.GracePeriod)
		if err != nil {
			return opts, fmt.Errorf("parsing Swarm.ConnMgr.GracePeriod: %s", err)
		}

		low = cfg.Swarm.ConnMgr.LowWater
		high = cfg.Swarm.ConnMgr.HighWater
	default:
		return opts, fmt.Errorf("unrecognized ConnMgr.Type: %q", cfg.Swarm.ConnMgr.Type)
	}

	cm := connmgr.NewConnManager(low, high, grace)
	opts.Opts = append(opts.Opts, libp2p.ConnectionManager(cm))
	return
}

func makeSmuxTransportOption(mplexExp bool) libp2p.Option {
	const yamuxID = "/yamux/1.0.0"
	const mplexID = "/mplex/6.7.0"

	ymxtpt := &yamux.Transport{
		AcceptBacklog:          512,
		ConnectionWriteTimeout: time.Second * 10,
		KeepAliveInterval:      time.Second * 30,
		EnableKeepAlive:        true,
		MaxStreamWindowSize:    uint32(16 * 1024 * 1024), // 16MiB
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

func P2PSmuxTransport(mplex bool) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		opts.Opts = append(opts.Opts, makeSmuxTransportOption(mplex))
		return
	}
}

func P2PNatPortMap(cfg *config.Config) (opts Libp2pOpts, err error) {
	if !cfg.Swarm.DisableNatPortMap {
		opts.Opts = append(opts.Opts, libp2p.NATPortMap())
	}
	return
}

func P2PRelay(cfg *config.Config) (opts Libp2pOpts, err error) {
	if cfg.Swarm.DisableRelay {
		// Enabled by default.
		opts.Opts = append(opts.Opts, libp2p.DisableRelay())
	} else {
		relayOpts := []relay.RelayOpt{relay.OptDiscovery}
		if cfg.Swarm.EnableRelayHop {
			relayOpts = append(relayOpts, relay.OptHop)
		}
		opts.Opts = append(opts.Opts, libp2p.EnableRelay(relayOpts...))
	}
	return
}

func P2PAutoRealy(cfg *config.Config) (opts Libp2pOpts, err error) {
	// enable autorelay
	if cfg.Swarm.EnableAutoRelay {
		opts.Opts = append(opts.Opts, libp2p.EnableAutoRelay())
	}
	return
}

func P2PDefaultTransports() (opts Libp2pOpts, err error) {
	opts.Opts = append(opts.Opts, libp2p.DefaultTransports)
	return
}

func P2PQUIC(cfg *config.Config) (opts Libp2pOpts, err error) {
	if cfg.Experimental.QUIC {
		opts.Opts = append(opts.Opts, libp2p.Transport(libp2pquic.NewTransport))
	}
	return
}

func P2PNoSecurity() (opts Libp2pOpts) {
	opts.Opts = append(opts.Opts, libp2p.NoSecurity)
	// TODO: shouldn't this be Errorf to guarantee visibility?
	log.Warningf(`Your IPFS node has been configured to run WITHOUT ENCRYPTED CONNECTIONS.
		You will not be able to connect to any nodes configured to use encrypted connections`)
	return opts
}

type P2PHostIn struct {
	fx.In

	Repo          repo.Repo
	Validator     record.Validator
	HostOption    HostOption
	RoutingOption RoutingOption
	ID            peer.ID
	Peerstore     peerstore.Peerstore

	Opts [][]libp2p.Option `group:"libp2p"`
}

type BaseRouting routing.IpfsRouting
type P2PHostOut struct {
	fx.Out

	Host    host.Host
	Routing BaseRouting
}

func P2PHost(mctx MetricsCtx, lc fx.Lifecycle, params P2PHostIn) (out P2PHostOut, err error) {
	opts := []libp2p.Option{libp2p.NoListenAddrs}
	for _, o := range params.Opts {
		opts = append(opts, o...)
	}

	ctx := lifecycleCtx(mctx, lc)

	opts = append(opts, libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		r, err := params.RoutingOption(ctx, h, params.Repo.Datastore(), params.Validator)
		out.Routing = r
		return r, err
	}))

	out.Host, err = params.HostOption(ctx, params.ID, params.Peerstore, opts...)
	if err != nil {
		return P2PHostOut{}, err
	}

	// this code is necessary just for tests: mock network constructions
	// ignore the libp2p constructor options that actually construct the routing!
	if out.Routing == nil {
		r, err := params.RoutingOption(ctx, out.Host, params.Repo.Datastore(), params.Validator)
		if err != nil {
			return P2PHostOut{}, err
		}
		out.Routing = r
		out.Host = routedhost.Wrap(out.Host, out.Routing)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return out.Host.Close()
		},
	})

	return out, err
}

type Router struct {
	routing.IpfsRouting

	Priority int // less = more important
}

type p2pRouterOut struct {
	fx.Out

	Router Router `group:"routers"`
}

func P2PBaseRouting(lc fx.Lifecycle, in BaseRouting) (out p2pRouterOut, dr *dht.IpfsDHT) {
	if dht, ok := in.(*dht.IpfsDHT); ok {
		dr = dht

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return dr.Close()
			},
		})
	}

	return p2pRouterOut{
		Router: Router{
			Priority:    1000,
			IpfsRouting: in,
		},
	}, dr
}

type p2pOnlineRoutingIn struct {
	fx.In

	Routers   []Router `group:"routers"`
	Validator record.Validator
}

func P2PRouting(in p2pOnlineRoutingIn) routing.IpfsRouting {
	routers := in.Routers

	sort.SliceStable(routers, func(i, j int) bool {
		return routers[i].Priority < routers[j].Priority
	})

	irouters := make([]routing.IpfsRouting, len(routers))
	for i, v := range routers {
		irouters[i] = v.IpfsRouting
	}

	return routinghelpers.Tiered{
		Routers:   irouters,
		Validator: in.Validator,
	}
}

type p2pPSRoutingIn struct {
	fx.In

	BaseRouting BaseRouting
	Repo        repo.Repo
	Validator   record.Validator
	Host        host.Host
	PubSub      *pubsub.PubSub `optional:"true"`
}

func P2PPubsubRouter(mctx MetricsCtx, lc fx.Lifecycle, in p2pPSRoutingIn) (p2pRouterOut, *namesys.PubsubValueStore) {
	psRouter := namesys.NewPubsubValueStore(
		lifecycleCtx(mctx, lc),
		in.Host,
		in.BaseRouting,
		in.PubSub,
		in.Validator,
	)

	return p2pRouterOut{
		Router: Router{
			IpfsRouting: &routinghelpers.Compose{
				ValueStore: &routinghelpers.LimitedValueStore{
					ValueStore: psRouter,
					Namespaces: []string{"ipns"},
				},
			},
			Priority: 100,
		},
	}, psRouter
}

func AutoNATService(mctx MetricsCtx, lc fx.Lifecycle, cfg *config.Config, host host.Host) error {
	if !cfg.Swarm.EnableAutoNATService {
		return nil
	}
	var opts []libp2p.Option
	if cfg.Experimental.QUIC {
		opts = append(opts, libp2p.DefaultTransports, libp2p.Transport(libp2pquic.NewTransport))
	}

	_, err := autonat.NewAutoNATService(lifecycleCtx(mctx, lc), host, opts...)
	return err
}

func Pubsub(mctx MetricsCtx, lc fx.Lifecycle, host host.Host, cfg *config.Config) (service *pubsub.PubSub, err error) {
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
		service, err = pubsub.NewFloodSub(lifecycleCtx(mctx, lc), host, pubsubOptions...)

	case "gossipsub":
		service, err = pubsub.NewGossipSub(lifecycleCtx(mctx, lc), host, pubsubOptions...)

	default:
		err = fmt.Errorf("Unknown pubsub router %s", cfg.Pubsub.Router)
	}

	return service, err
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

func StartListening(host host.Host, cfg *config.Config) error {
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
