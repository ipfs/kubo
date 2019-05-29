package coreapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	ds "github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	config "github.com/ipfs/go-ipfs-config"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	util "github.com/ipfs/go-ipfs-util"
	"github.com/ipfs/go-metrics-interface"
	"github.com/ipfs/go-path/resolver"
	uio "github.com/ipfs/go-unixfs/io"
	"github.com/jbenet/goprocess"
	ci "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/node"
	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/keystore"
	"github.com/ipfs/go-ipfs/p2p"
	"github.com/ipfs/go-ipfs/provider"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/reprovide"
)

// from go-ipfs-config/init.go
var defaultListenAddrs = []string{ // TODO: we need more than 1, better defaults?
	"/ip4/0.0.0.0/tcp/4001",
	"/ip6/::/tcp/4001",
}

const (
	// TODO: docstrings on everything
	// TODO: think about not exporting some of this (only export stable-ish stuff)

	Goprocess component = iota

	// baseRepo can be overridden externally with the Repo option
	baseRepo
	Config
	BatchDatastore
	Datastore
	BlockstoreBasic
	BlockstoreFinal

	Peerid
	PrivKey
	PubKey
	Peerstore
	PeerstoreAddSelf

	Validator
	Router
	Namesys
	IpnsRepublisher

	ProviderKeys
	ProviderQueue
	Reprovider
	ReproviderSvc
	Provider
	Exchange

	Blockservice
	Dag
	Pinning
	Files
	Resolver

	P2PTunnel

	FxLogger

	Libp2pPrivateNetwork
	Libp2pPrivateNetworkChecker
	Libp2pDefaultTransports
	Libp2pHost
	Libp2pRoutedHost
	Libp2pDiscoveryHandler

	Libp2pAddrFilters
	Libp2pMDNS         // SetupDiscovery
	Libp2pAddrsFactory // TODO: better name?
	Libp2pSmuxTransport
	Libp2pRelay
	Libp2pStartListening

	Libp2pSecurity

	Libp2pRouting
	Libp2pBaseRouting
	Libp2pPubsubRouter

	Libp2pBandwidthCounter
	Libp2pNatPortMap
	Libp2pAutoRealy
	Libp2pQUIC
	Libp2pAutoNATService
	Libp2pConnectionManager
	Libp2pPubsub

	nComponents // MUST be last
)

type component int

type settings struct {
	components []fx.Option
	userFx     []fx.Option

	ctx context.Context

	online  bool // TODO: migrate to components fully
	nilRepo bool // TODO: try not to use, somehow (component-internal fields?)
}

type Option func(*settings) error

// ////////// //
// Low-level options

func Provide(i interface{}) Option {
	return func(s *settings) error {
		s.userFx = append(s.userFx, fx.Provide(i))
		return nil
	}
}

// TODO: docstring warning that this method is powerful but dangerous
func Override(c component, replacement interface{}, pf ...func(...interface{}) fx.Option) Option {
	if reflect.TypeOf(replacement).Kind() != reflect.Func {
		panic("override replacement not a func")
	}

	return func(s *settings) error {
		if len(pf) == 0 {
			pf = []func(...interface{}) fx.Option{fx.Provide}
		}
		if len(pf) != 1 {
			return errors.New("Invalid number of Override args")
		}

		s.components[c] = pf[0](replacement)

		// TODO: might do type checking with reflection, but it's probably not worth it
		// TODO: it might be actually very useful for documentation purposes
		return nil
	}
}

func Options(opts ...Option) Option {
	return func(s *settings) error {
		for _, opt := range opts {
			if err := opt(s); err != nil {
				return err
			}
		}
		return nil
	}
}

// ifNil checks if a component is already set, and if not, applies options
func ifNil(c component, f ...Option) Option {
	return func(s *settings) error {
		if s.components[c] == nil {
			return Options(f...)(s)
		}
		return nil
	}
}

func ifSet(c component, f ...Option) Option {
	return func(s *settings) error {
		if s.components[c] != nil {
			return Options(f...)(s)
		}
		return nil
	}
}

func errOpt(err error) Option {
	return func(_ *settings) error {
		return err
	}
}

func Opt(cond bool, opt Option) Option {
	if cond {
		return opt
	}
	return Options()
}

// ////////// //
// Core options

func Ctx(ctx context.Context) Option {
	return func(s *settings) error {
		s.ctx = ctx
		return nil
	}
}

// ////////// //
// User options

func NilRepo() Option {
	return func(s *settings) error {
		s.nilRepo = true
		return Override(BatchDatastore, as(ds.NewNullDatastore, new(ds.Batching)))(s)
	}
}

func Online() Option {
	lgcOnline := func(s *settings) error {
		s.online = true
		return nil
	}

	return Options(lgcOnline,
		ifNil(PrivKey,
			RandomIdentity(),
		),
		Override(PeerstoreAddSelf, libp2p.PstoreAddSelfKeys, fx.Invoke),

		Override(Exchange, node.OnlineExchange),

		Override(Namesys, node.Namesys(node.DefaultIpnsCacheSize)),
		Override(IpnsRepublisher, node.IpnsRepublisher(0, 0), fx.Invoke), // TODO: verify defaults (might be set in go-ipfs-config)

		Override(P2PTunnel, p2p.New),

		Override(Provider, node.ProviderCtor),
		Override(ProviderQueue, node.ProviderQueue),
		Override(ProviderKeys, reprovide.NewBlockstoreProvider),
		Override(Reprovider, node.ReproviderCtor(node.DefaultReprovideFrequency)),
		Override(ReproviderSvc, node.Reprovider, fx.Invoke),

		// LibP2P

		Override(Libp2pDefaultTransports, libp2p.DefaultTransports),
		Override(Libp2pHost, libp2p.Host),
		Override(Libp2pRoutedHost, libp2p.RoutedHost),
		Override(Libp2pRouting, libp2p.DHTRouting(true)), // default dhtclient

		Override(Libp2pDiscoveryHandler, libp2p.DiscoveryHandler),
		Override(Libp2pAddrsFactory, libp2p.AddrsFactory(nil, nil)),
		Override(Libp2pSmuxTransport, libp2p.SmuxTransport(true)),
		Override(Libp2pRelay, libp2p.Relay(true, false)), // TODO: should we enable by default?
		Override(Libp2pStartListening, libp2p.StartListening(defaultListenAddrs), fx.Invoke),
		Override(Libp2pSecurity, libp2p.Security(true, false)), // enabled, prefer secio
		Override(Router, libp2p.Routing),
		Override(Libp2pBaseRouting, libp2p.BaseRouting),
		Override(Libp2pNatPortMap, libp2p.NatPortMap),
		Override(Libp2pConnectionManager, libp2p.ConnectionManager(config.DefaultConnMgrLowWater, config.DefaultConnMgrHighWater, config.DefaultConnMgrGracePeriod)),
	)
}

// ////////// //
// Repo / config

func configIdentity(ident config.Identity) Option {
	cid := ident.PeerID
	if cid == "" {
		return errOpt(errors.New("identity was not set in config (was 'ipfs init' run?)"))
	}
	if len(cid) == 0 {
		return errOpt(errors.New("no peer ID in config! (was 'ipfs init' run?)"))
	}

	id, err := peer.IDB58Decode(cid)
	if err != nil {
		return errOpt(fmt.Errorf("peer ID invalid: %s", err))
	}

	// Private Key

	if ident.PrivKey == "" {
		return Options( // No PK (usually in tests)
			Override(Peerid, as(id, new(peer.ID))),
		)
	}

	sk, err := ident.DecodePrivateKey("passphrase todo!")
	if err != nil {
		return errOpt(err)
	}

	return Options(
		Override(Peerid, as(id, new(peer.ID))),
		Override(PrivKey, as(sk, new(ci.PrivKey))),
		Override(PubKey, as(sk.GetPublic(), new(ci.PubKey))),
	)
}

func configDatastore(dstore config.Datastore, s *repoSettings) Option {
	cacheOpts := blockstore.DefaultCacheOpts()
	cacheOpts.HasBloomFilterSize = dstore.BloomFilterSize
	if !s.permanent {
		cacheOpts.HasBloomFilterSize = 0
	}

	return Options(
		Override(BlockstoreBasic, node.BaseBlockstoreCtor(cacheOpts, s.nilRepo, dstore.HashOnRead)),
	)
}

func configAddresses(addrs config.Addresses) Option {
	return ifSet(Libp2pHost,
		Override(Libp2pStartListening, libp2p.StartListening(addrs.Swarm), fx.Invoke),
		Override(Libp2pAddrsFactory, libp2p.AddrsFactory(addrs.Announce, addrs.NoAnnounce)),
	)
}

func configDiscovery(disc config.Discovery) Option {
	return ifSet(Libp2pHost, Override(Libp2pMDNS, libp2p.SetupDiscovery(disc.MDNS.Enabled, disc.MDNS.Interval), fx.Invoke))
}

const (
	routingOptionDHTClient = "dhtclient"
	routingOptionDHT       = "dht"
	routingOptionNone      = "none"
)

func configRouting(routing config.Routing) Option {
	switch routing.Type {
	case "":
		return Options() // keep default
	case routingOptionDHT: // default
		return Override(Libp2pRouting, libp2p.DHTRouting(false))
	case routingOptionDHTClient:
		return Override(Libp2pRouting, libp2p.DHTRouting(true))
	case routingOptionNone:
		return Override(Libp2pRouting, libp2p.NilRouting)
	default:
		return errOpt(errors.New("unknown Routing.Type in config"))
	}
}

func configIpns(ipns config.Ipns) Option {
	ipnsCacheSize := ipns.ResolveCacheSize
	if ipnsCacheSize == 0 {
		ipnsCacheSize = node.DefaultIpnsCacheSize
	}
	if ipnsCacheSize < 0 {
		return errOpt(fmt.Errorf("cannot specify negative resolve cache size"))
	}

	// Republisher params

	var repubPeriod, recordLifetime time.Duration

	if ipns.RepublishPeriod != "" {
		d, err := time.ParseDuration(ipns.RepublishPeriod)
		if err != nil {
			return errOpt(fmt.Errorf("failure to parse config setting IPNS.RepublishPeriod: %s", err))
		}

		if !util.Debug && (d < time.Minute || d > (time.Hour*24)) {
			return errOpt(fmt.Errorf("config setting IPNS.RepublishPeriod is not between 1min and 1day: %s", d))
		}

		repubPeriod = d
	}

	if ipns.RecordLifetime != "" {
		d, err := time.ParseDuration(ipns.RecordLifetime)
		if err != nil {
			return errOpt(fmt.Errorf("failure to parse config setting IPNS.RecordLifetime: %s", err))
		}

		recordLifetime = d
	}

	return ifSet(Libp2pRouting,
		Override(Namesys, node.Namesys(ipnsCacheSize)),
		Override(IpnsRepublisher, node.IpnsRepublisher(repubPeriod, recordLifetime), fx.Invoke),
	)
}

/*
TODO: part bootstrap system to new node
func configBootstrap(bootstrap []string) Option {

}
*/

func configSwarm(swarm config.SwarmConfig, exp config.Experiments) Option {
	grace := config.DefaultConnMgrGracePeriod
	low := config.DefaultConnMgrLowWater
	high := config.DefaultConnMgrHighWater

	connmgr := Options()

	if swarm.ConnMgr.Type != "none" {
		switch swarm.ConnMgr.Type {
		case "":
			// 'default' value is the basic connection manager
			break
		case "basic":
			var err error
			grace, err = time.ParseDuration(swarm.ConnMgr.GracePeriod)
			if err != nil {
				return errOpt(fmt.Errorf("parsing Swarm.ConnMgr.GracePeriod: %s", err))
			}

			low = swarm.ConnMgr.LowWater
			high = swarm.ConnMgr.HighWater
		default:
			return errOpt(fmt.Errorf("unrecognized ConnMgr.Type: %q", swarm.ConnMgr.Type))
		}

		connmgr = Override(Libp2pConnectionManager, libp2p.ConnectionManager(low, high, grace))
	}

	return Options(
		Override(Libp2pAddrFilters, libp2p.AddrFilters(swarm.AddrFilters)),
		Opt(!swarm.DisableBandwidthMetrics, Override(Libp2pBandwidthCounter, libp2p.BandwidthCounter)),
		Opt(!swarm.DisableNatPortMap, Override(Libp2pNatPortMap, libp2p.NatPortMap)),
		Override(Libp2pRelay, libp2p.Relay(swarm.DisableRelay, swarm.EnableRelayHop)),
		Opt(swarm.EnableAutoRelay, Override(Libp2pAutoRealy, libp2p.AutoRelay)),
		Opt(swarm.EnableAutoNATService, Override(Libp2pAutoRealy, libp2p.AutoNATService(exp.QUIC))),
		connmgr,
	)
}

func configPubsub(ps config.PubsubConfig) Option {
	var pubsubOptions []pubsub.Option
	if ps.DisableSigning {
		pubsubOptions = append(pubsubOptions, pubsub.WithMessageSigning(false))
	}

	if ps.StrictSignatureVerification {
		pubsubOptions = append(pubsubOptions, pubsub.WithStrictSignatureVerification(true))
	}

	switch ps.Router {
	case "":
		fallthrough
	case "floodsub":
		return Override(Libp2pPubsub, libp2p.FloodSub(pubsubOptions...))
	case "gossipsub":
		return Override(Libp2pPubsub, libp2p.GossipSub(pubsubOptions...))
	default:
		return errOpt(fmt.Errorf("unknown pubsub router %s", ps.Router))
	}
}

func configReprovider(reprovider config.Reprovider) Option {
	reproviderInterval := node.DefaultReprovideFrequency
	if reprovider.Interval != "" {
		dur, err := time.ParseDuration(reprovider.Interval)
		if err != nil {
			return errOpt(err)
		}

		reproviderInterval = dur
	}

	var keyProvider interface{}
	switch reprovider.Strategy {
	case "all":
		fallthrough
	case "":
		keyProvider = reprovide.NewBlockstoreProvider
	case "roots":
		keyProvider = reprovide.NewPinnedProvider(true)
	case "pinned":
		keyProvider = reprovide.NewPinnedProvider(false)
	default:
		return errOpt(fmt.Errorf("unknown reprovider strategy '%s'", reprovider.Strategy))
	}

	return Options(
		Override(Reprovider, node.ReproviderCtor(reproviderInterval)),
		Override(ProviderKeys, keyProvider),
	)
}

func configExperimental(experiments config.Experiments) Option {
	fsbs := experiments.FilestoreEnabled || experiments.UrlstoreEnabled

	// TODO: Eww
	uio.UseHAMTSharding = experiments.ShardingEnabled

	return Options(
		Opt(fsbs, Override(BlockstoreFinal, node.FilestoreBlockstoreCtor)),
		Opt(experiments.QUIC, Override(Libp2pQUIC, libp2p.QUIC)),
		Override(Libp2pSecurity, libp2p.Security(true, experiments.PreferTLS)),
	)
}

func configOptions(cfg *config.Config, s *repoSettings) Option {
	// Identity
	return Options(
		configIdentity(cfg.Identity),
		configDatastore(cfg.Datastore, s),
		configAddresses(cfg.Addresses),
		configDiscovery(cfg.Discovery),
		configRouting(cfg.Routing),
		configIpns(cfg.Ipns),
		configSwarm(cfg.Swarm, cfg.Experimental),
		configPubsub(cfg.Pubsub),
		configReprovider(cfg.Reprovider),
		configExperimental(cfg.Experimental),
	)
}

type repoSettings struct {
	parseConfig bool
	permanent   bool
	nilRepo     bool
}

type RepoOption func(*repoSettings)

// TODO: should we invert this option (SkipConfig?)
func ParseConfig(s *repoSettings) {
	s.parseConfig = true
}

// TODO: better name? (this is only enabling bloom filter if set in config)
func Permanent(s *repoSettings) {
	s.permanent = true
}

func Repo(r repo.Repo, opts ...RepoOption) Option {
	return func(s *settings) error {
		rs := &repoSettings{
			nilRepo: s.nilRepo,
		}
		for _, opt := range opts {
			opt(rs)
		}

		repoOption := Override(baseRepo, func() repo.Repo { return r })
		if !rs.parseConfig {
			return repoOption(s)
		}

		cfg, err := r.Config()
		if err != nil {
			return err
		}

		return Options(repoOption, configOptions(cfg, rs))(s)
	}
}

// ////////// //
// Misc / test options

func RandomIdentity() Option {
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		return errOpt(err)
	}

	return Options(
		Override(PrivKey, as(sk, new(ci.PrivKey))),
		Override(PubKey, as(pk, new(ci.PubKey))),
		Override(Peerid, peer.IDFromPublicKey),
	)
}

// ////////// //
// Constructor

func defaults() settings {
	out := settings{
		components: make([]fx.Option, nComponents),

		ctx: context.Background(),
	}

	out.components[Goprocess] = fx.Provide(baseProcess)
	out.components[baseRepo] = fx.Provide(memRepo)

	out.components[Config] = fx.Provide(repo.Repo.Config)
	out.components[BatchDatastore] = fx.Provide(repo.Repo.Datastore)
	out.components[Datastore] = fx.Provide(node.RawDatastore)
	out.components[BlockstoreBasic] = fx.Provide(node.BaseBlockstoreCtor(blockstore.DefaultCacheOpts(), false, false))
	out.components[BlockstoreFinal] = fx.Provide(node.GcBlockstoreCtor)

	out.components[Peerid] = fx.Provide(node.RandomPeerID)
	out.components[Peerstore] = fx.Provide(pstoremem.NewPeerstore) // privkey / addself

	out.components[Validator] = fx.Provide(node.RecordValidator)
	out.components[Router] = fx.Provide(offroute.NewOfflineRouter)
	out.components[Provider] = fx.Provide(provider.NewOfflineProvider)

	out.components[Exchange] = fx.Provide(offline.Exchange)
	out.components[Namesys] = fx.Provide(node.Namesys(0))
	out.components[Blockservice] = fx.Provide(node.BlockService)
	out.components[Dag] = fx.Provide(node.Dag)
	out.components[Pinning] = fx.Provide(node.Pinning)
	out.components[Files] = fx.Provide(node.Files)
	out.components[Resolver] = fx.Provide(resolver.NewBasicResolver)

	out.components[FxLogger] = fx.NopLogger

	return out
}

func New(opts ...Option) (*CoreAPI, error) {
	settings := defaults()
	if err := Options(opts...)(&settings); err != nil {
		return nil, err
	}

	ctx := metrics.CtxScope(settings.ctx, "ipfs")
	fxOpts := make([]fx.Option, len(settings.userFx)+len(settings.components)+2)
	for i, opt := range settings.userFx {
		if opt == nil {
			opt = fx.Options()
		}
		fxOpts[i] = opt
	}

	for i, opt := range settings.components {
		if opt == nil {
			opt = fx.Options()
		}
		fxOpts[i+len(settings.userFx)] = opt
	}

	n := &core.IpfsNode{}

	fxOpts[len(fxOpts)-2] = fx.Provide(func() helpers.MetricsCtx {
		return helpers.MetricsCtx(ctx)
	})
	fxOpts[len(fxOpts)-1] = fx.Extract(n)

	app := fx.New(fxOpts...)
	n.SetupCtx(ctx, app)
	n.IsOnline = settings.online

	go func() {
		// Note that some services use contexts to signal shutting down, which is
		// very suboptimal. This needs to be here until that's addressed somehow
		<-ctx.Done()
		app.Stop(context.Background())
	}()

	if err := app.Start(ctx); err != nil {
		return nil, err
	}

	// TEMP: setting global sharding switch here
	//TODO uio.UseHAMTSharding = cfg.Experimental.ShardingEnabled

	if settings.online { // TODO: use components
		if err := n.Bootstrap(bootstrap.DefaultBootstrapConfig); err != nil {
			return nil, err
		}
	}

	return NewCoreAPI(n)
}

// ////////// //
// Utils

func memRepo() repo.Repo {
	c := config.Config{}
	// c.PrivKey = ident //TODO, probably
	c.Experimental.FilestoreEnabled = true

	ds := ds.NewMapDatastore()
	return &repo.Mock{
		C: c,
		D: syncds.MutexWrap(ds),
		K: keystore.NewMemKeystore(),
		F: filestore.NewFileManager(ds, filepath.Dir(os.TempDir())),
	}
}

// copied from old node pkg

// baseProcess creates a goprocess which is closed when the lifecycle signals it to stop
func baseProcess(lc fx.Lifecycle) goprocess.Process {
	p := goprocess.WithParent(goprocess.Background())
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return p.Close()
		},
	})
	return p
}

// as casts input constructor to a given interface (if a value is given, it
// wraps it into a constructor).
//
// Note: this method may look like a hack, and in fact it is one.
// This is here only because https://github.com/uber-go/fx/issues/673 wasn't
// released yet
//
// Note 2: when making changes here, make sure this method stays at
// 100% coverage. This makes it less likely it will be terribly broken
func as(in interface{}, as interface{}) interface{} {
	outType := reflect.TypeOf(as)

	if outType.Kind() != reflect.Ptr {
		panic("outType is not a pointer")
	}

	if reflect.TypeOf(in).Kind() != reflect.Func {
		ctype := reflect.FuncOf(nil, []reflect.Type{outType.Elem()}, false)

		return reflect.MakeFunc(ctype, func(args []reflect.Value) (results []reflect.Value) {
			out := reflect.New(outType.Elem())
			out.Elem().Set(reflect.ValueOf(in))

			return []reflect.Value{out.Elem()}
		}).Interface()
	}

	inType := reflect.TypeOf(in)

	ins := make([]reflect.Type, inType.NumIn())
	outs := make([]reflect.Type, inType.NumOut())

	for i := range ins {
		ins[i] = inType.In(i)
	}
	outs[0] = outType.Elem()
	for i := range outs[1:] {
		outs[i+1] = inType.Out(i + 1)
	}

	ctype := reflect.FuncOf(ins, outs, false)

	return reflect.MakeFunc(ctype, func(args []reflect.Value) (results []reflect.Value) {
		outs := reflect.ValueOf(in).Call(args)
		out := reflect.New(outType.Elem())
		out.Elem().Set(outs[0])
		outs[0] = out.Elem()

		return outs
	}).Interface()
}
