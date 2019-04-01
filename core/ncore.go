package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/ipfs/go-ipfs-exchange-offline"
	u "github.com/ipfs/go-ipfs-util"
	rp "github.com/ipfs/go-ipfs/exchange/reprovide"
	"github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/namesys"
	ipnsrp "github.com/ipfs/go-ipfs/namesys/republisher"
	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/provider"
	"github.com/ipfs/go-ipfs/thirdparty/cidv0v1"
	"github.com/ipfs/go-ipfs/thirdparty/verifbs"
	"github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-ipns"
	merkledag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-mfs"
	ft "github.com/ipfs/go-unixfs"
	"github.com/jbenet/goprocess"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-autonat-svc"
	circuit "github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-metrics"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"github.com/libp2p/go-libp2p-pnet"
	"github.com/libp2p/go-libp2p-pubsub"
	psrouter "github.com/libp2p/go-libp2p-pubsub-router"
	quic "github.com/libp2p/go-libp2p-quic-transport"
	"github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p-routing"
	rhelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"go.uber.org/fx"
	"time"

	"github.com/ipfs/go-ipfs/repo"

	retry "github.com/ipfs/go-datastore/retrystore"
	iconfig "github.com/ipfs/go-ipfs-config"
	uio "github.com/ipfs/go-unixfs/io"
	ic "github.com/libp2p/go-libp2p-crypto"
	p2phost "github.com/libp2p/go-libp2p-host"
	"github.com/libp2p/go-libp2p-peer"
	mamask "github.com/whyrusleeping/multiaddr-filter"
)

func repoConfig(repo repo.Repo) (*iconfig.Config, error) {
	return repo.Config()
}

func identity(cfg *iconfig.Config) (peer.ID, error) {
	cid := cfg.Identity.PeerID
	if cid == "" {
		return "", errors.New("identity was not set in config (was 'ipfs init' run?)")
	}
	if len(cid) == 0 {
		return "", errors.New("no peer ID in config! (was 'ipfs init' run?)")
	}

	id, err := peer.IDB58Decode(cid)
	if err != nil {
		return "", fmt.Errorf("peer ID invalid: %s", err)
	}

	return id, nil
}

func peerstore(id peer.ID, sk ic.PrivKey) pstore.Peerstore {
	ps := pstoremem.NewPeerstore()

	if sk != nil {
		ps.AddPrivKey(id, sk)
		ps.AddPubKey(id, sk.GetPublic())
	}

	return ps
}

func privateKey(cfg *iconfig.Config, id peer.ID) (ic.PrivKey, error) {
	if cfg.Identity.PrivKey == "" {
		return nil, nil
	}

	sk, err := cfg.Identity.DecodePrivateKey("passphrase todo!")
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

func datastoreCtor(repo repo.Repo) ds.Datastore {
	return repo.Datastore()
}

type BaseBlocks bstore.Blockstore

func baseBlockstoreCtor(mctx MetricsCtx, repo repo.Repo, cfg *iconfig.Config, bcfg *BuildCfg, lc fx.Lifecycle) (bs BaseBlocks, err error) {
	rds := &retry.Datastore{
		Batching:    repo.Datastore(),
		Delay:       time.Millisecond * 200,
		Retries:     6,
		TempErrFunc: isTooManyFDError,
	}
	// hash security
	bs = bstore.NewBlockstore(rds)
	bs = &verifbs.VerifBS{Blockstore: bs}

	opts := bstore.DefaultCacheOpts()
	opts.HasBloomFilterSize = cfg.Datastore.BloomFilterSize
	if !bcfg.Permanent {
		opts.HasBloomFilterSize = 0
	}

	if !bcfg.NilRepo {
		ctx, cancel := context.WithCancel(mctx)

		lc.Append(fx.Hook{
			OnStop: func(context context.Context) error {
				cancel()
				return nil
			},
		})
		bs, err = bstore.CachedBlockstore(ctx, bs, opts)
		if err != nil {
			return nil, err
		}
	}

	bs = bstore.NewIdStore(bs)
	bs = cidv0v1.NewBlockstore(bs)

	if cfg.Datastore.HashOnRead { // TODO: review: this is how it was done originally, is there a reason we can't just pass this directly?
		bs.HashOnRead(true)
	}

	return
}

func gcBlockstoreCtor(repo repo.Repo, bb BaseBlocks, cfg *iconfig.Config) (gclocker bstore.GCLocker, gcbs bstore.GCBlockstore, bs bstore.Blockstore, fstore *filestore.Filestore) {
	gclocker = bstore.NewGCLocker()
	gcbs = bstore.NewGCBlockstore(bb, gclocker)

	if cfg.Experimental.FilestoreEnabled || cfg.Experimental.UrlstoreEnabled {
		// hash security
		fstore = filestore.NewFilestore(bb, repo.FileManager()) //TODO: mark optional
		gcbs = bstore.NewGCBlockstore(fstore, gclocker)
		gcbs = &verifbs.VerifBSGC{GCBlockstore: gcbs}
	}
	bs = gcbs
	return
}

func recordValidator(ps pstore.Peerstore) record.Validator {
	return record.NamespacedValidator{
		"pk":   record.PublicKeyValidator{},
		"ipns": ipns.Validator{KeyBook: ps},
	}
}

////////////////////
// libp2p related

////////////////////
// libp2p

var ipfsp2p = fx.Options(
	fx.Provide(p2pAddrFilters),
	fx.Provide(p2pBandwidthCounter),
	fx.Provide(p2pPNet),
	fx.Provide(p2pAddrsFactory),
	fx.Provide(p2pConnectionManager),
	fx.Provide(p2pSmuxTransport),
	fx.Provide(p2pNatPortMap),
	fx.Provide(p2pRelay),
	fx.Provide(p2pAutoRealy),
	fx.Provide(p2pDefaultTransports),
	fx.Provide(p2pQUIC),

	fx.Provide(p2pHostOption),
	fx.Provide(p2pHost),
	fx.Provide(p2pOnlineRouting),

	fx.Provide(pubsubCtor),
	fx.Provide(newDiscoveryHandler),

	fx.Invoke(autoNATService),
	fx.Invoke(p2pPNetChecker),
	fx.Invoke(startListening),
	fx.Invoke(setupDiscovery),
)

func p2pHostOption(bcfg *BuildCfg) (hostOption HostOption, err error) {
	hostOption = bcfg.Host
	if bcfg.DisableEncryptedConnections {
		innerHostOption := hostOption
		hostOption = func(ctx context.Context, id peer.ID, ps pstore.Peerstore, options ...libp2p.Option) (p2phost.Host, error) {
			return innerHostOption(ctx, id, ps, append(options, libp2p.NoSecurity)...)
		}
		// TODO: shouldn't this be Errorf to guarantee visibility?
		log.Warningf(`Your IPFS node has been configured to run WITHOUT ENCRYPTED CONNECTIONS.
		You will not be able to connect to any nodes configured to use encrypted connections`)
	}
	return hostOption, nil
}

func p2pAddrFilters(cfg *iconfig.Config) (opts libp2pOpts, err error) {
	for _, s := range cfg.Swarm.AddrFilters {
		f, err := mamask.NewMask(s)
		if err != nil {
			return opts, fmt.Errorf("incorrectly formatted address filter in config: %s", s)
		}
		opts.Opts = append(opts.Opts, libp2p.FilterAddresses(f))
	}
	return opts, nil
}

func p2pBandwidthCounter(cfg *iconfig.Config) (opts libp2pOpts, reporter metrics.Reporter) {
	reporter = metrics.NewBandwidthCounter()

	if !cfg.Swarm.DisableBandwidthMetrics {
		opts.Opts = append(opts.Opts, libp2p.BandwidthReporter(reporter))
	}
	return opts, reporter
}

type libp2pOpts struct {
	fx.Out

	Opts []libp2p.Option `group:"libp2p"`
}

type PNetFingerprint []byte // TODO: find some better place
func p2pPNet(repo repo.Repo) (opts libp2pOpts, fp PNetFingerprint, err error) {
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

func p2pPNetChecker(repo repo.Repo, ph p2phost.Host, lc fx.Lifecycle) error {
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

func p2pAddrsFactory(cfg *iconfig.Config) (opts libp2pOpts, err error) {
	addrsFactory, err := makeAddrsFactory(cfg.Addresses)
	if err != nil {
		return opts, err
	}
	if !cfg.Swarm.DisableRelay {
		addrsFactory = composeAddrsFactory(addrsFactory, filterRelayAddrs)
	}
	opts.Opts = append(opts.Opts, libp2p.AddrsFactory(addrsFactory))
	return
}

func p2pConnectionManager(cfg *iconfig.Config) (opts libp2pOpts, err error) {
	connm, err := constructConnMgr(cfg.Swarm.ConnMgr)
	if err != nil {
		return opts, err
	}

	opts.Opts = append(opts.Opts, libp2p.ConnectionManager(connm))
	return
}

func p2pSmuxTransport(bcfg *BuildCfg) (opts libp2pOpts, err error) {
	opts.Opts = append(opts.Opts, makeSmuxTransportOption(bcfg.getOpt("mplex")))
	return
}

func p2pNatPortMap(cfg *iconfig.Config) (opts libp2pOpts, err error) {
	if !cfg.Swarm.DisableNatPortMap {
		opts.Opts = append(opts.Opts, libp2p.NATPortMap())
	}
	return
}

func p2pRelay(cfg *iconfig.Config) (opts libp2pOpts, err error) {
	if cfg.Swarm.DisableRelay {
		// Enabled by default.
		opts.Opts = append(opts.Opts, libp2p.DisableRelay())
	} else {
		relayOpts := []circuit.RelayOpt{circuit.OptDiscovery}
		if cfg.Swarm.EnableRelayHop {
			relayOpts = append(relayOpts, circuit.OptHop)
		}
		opts.Opts = append(opts.Opts, libp2p.EnableRelay(relayOpts...))
	}
	return
}

func p2pAutoRealy(cfg *iconfig.Config) (opts libp2pOpts, err error) {
	// enable autorelay
	if cfg.Swarm.EnableAutoRelay {
		opts.Opts = append(opts.Opts, libp2p.EnableAutoRelay())
	}
	return
}

func p2pDefaultTransports() (opts libp2pOpts, err error) {
	opts.Opts = append(opts.Opts, libp2p.DefaultTransports)
	return
}

func p2pQUIC(cfg *iconfig.Config) (opts libp2pOpts, err error) {
	if cfg.Experimental.QUIC {
		opts.Opts = append(opts.Opts, libp2p.Transport(quic.NewTransport))
	}
	return
}

type p2pHostIn struct {
	fx.In

	BCfg       *BuildCfg
	Repo       repo.Repo
	Validator  record.Validator
	HostOption HostOption
	ID         peer.ID
	Peerstore  pstore.Peerstore

	Opts [][]libp2p.Option `group:"libp2p"`
}

type BaseRouting routing.IpfsRouting
type p2pHostOut struct {
	fx.Out

	Host    p2phost.Host
	Routing BaseRouting
	IpfsDHT *dht.IpfsDHT
}

// TODO: move some of this into params struct
func p2pHost(mctx MetricsCtx, lc fx.Lifecycle, params p2pHostIn) (out p2pHostOut, err error) {
	opts := []libp2p.Option{libp2p.NoListenAddrs}
	for _, o := range params.Opts {
		opts = append(opts, o...)
	}

	ctx, cancel := context.WithCancel(mctx)
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})

	opts = append(opts, libp2p.Routing(func(h p2phost.Host) (routing.PeerRouting, error) {
		r, err := params.BCfg.Routing(ctx, h, params.Repo.Datastore(), params.Validator)
		out.Routing = r
		return r, err
	}))

	out.Host, err = params.HostOption(ctx, params.ID, params.Peerstore, opts...)
	if err != nil {
		return p2pHostOut{}, err
	}

	// this code is necessary just for tests: mock network constructions
	// ignore the libp2p constructor options that actually construct the routing!
	if out.Routing == nil {
		r, err := params.BCfg.Routing(ctx, out.Host, params.Repo.Datastore(), params.Validator)
		if err != nil {
			return p2pHostOut{}, err
		}
		out.Routing = r
		out.Host = rhost.Wrap(out.Host, out.Routing)
	}

	// TODO: break this up into more DI units
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
	if dht, ok := out.Routing.(*dht.IpfsDHT); ok {
		out.IpfsDHT = dht
	}

	return out, err
}

type p2pRoutingIn struct {
	fx.In

	BCfg      *BuildCfg
	Repo      repo.Repo
	Validator record.Validator
	Host      p2phost.Host
	PubSub    *pubsub.PubSub

	BaseRouting BaseRouting
}

type p2pRoutingOut struct {
	fx.Out

	IpfsRouting routing.IpfsRouting
	PSRouter    *psrouter.PubsubValueStore //TODO: optional
}

func p2pOnlineRouting(mctx MetricsCtx, lc fx.Lifecycle, in p2pRoutingIn) (out p2pRoutingOut) {
	out.IpfsRouting = in.BaseRouting

	if in.BCfg.getOpt("ipnsps") {
		out.PSRouter = psrouter.NewPubsubValueStore(
			lifecycleCtx(mctx, lc),
			in.Host,
			in.BaseRouting,
			in.PubSub,
			in.Validator,
		)

		out.IpfsRouting = rhelpers.Tiered{
			Routers: []routing.IpfsRouting{
				// Always check pubsub first.
				&rhelpers.Compose{
					ValueStore: &rhelpers.LimitedValueStore{
						ValueStore: out.PSRouter,
						Namespaces: []string{"ipns"},
					},
				},
				in.BaseRouting,
			},
			Validator: in.Validator,
		}
	}
	return out
}

////////////
// P2P services

func autoNATService(mctx MetricsCtx, lc fx.Lifecycle, cfg *iconfig.Config, host p2phost.Host) error {
	if !cfg.Swarm.EnableAutoNATService {
		return nil
	}
	var opts []libp2p.Option
	if cfg.Experimental.QUIC {
		opts = append(opts, libp2p.DefaultTransports, libp2p.Transport(quic.NewTransport))
	}

	_, err := autonat.NewAutoNATService(lifecycleCtx(mctx, lc), host, opts...)
	return err
}

func pubsubCtor(mctx MetricsCtx, lc fx.Lifecycle, host p2phost.Host, bcfg *BuildCfg, cfg *iconfig.Config) (service *pubsub.PubSub, err error) {
	if !(bcfg.getOpt("pubsub") || bcfg.getOpt("ipnsps")) {
		return nil, nil // TODO: mark optional
	}

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

////////////
// Offline services

// offline.Exchange
// offroute.NewOfflineRouter

func offlineNamesysCtor(rt routing.IpfsRouting, repo repo.Repo) (namesys.NameSystem, error) {
	return namesys.NewNameSystem(rt, repo.Datastore(), 0), nil
}

////////////
// IPFS services

func pinning(bstore bstore.Blockstore, ds format.DAGService, repo repo.Repo) (pin.Pinner, error) {
	internalDag := merkledag.NewDAGService(bserv.New(bstore, offline.Exchange(bstore)))
	pinning, err := pin.LoadPinner(repo.Datastore(), ds, internalDag)
	if err != nil {
		// TODO: we should move towards only running 'NewPinner' explicitly on
		// node init instead of implicitly here as a result of the pinner keys
		// not being found in the datastore.
		// this is kinda sketchy and could cause data loss
		pinning = pin.NewPinner(repo.Datastore(), ds, internalDag)
	}

	return pinning, nil
}

func dagCtor(bs bserv.BlockService) format.DAGService {
	return merkledag.NewDAGService(bs)
}

func onlineExchangeCtor(mctx MetricsCtx, lc fx.Lifecycle, host p2phost.Host, rt routing.IpfsRouting, bs bstore.GCBlockstore) exchange.Interface {
	bitswapNetwork := bsnet.NewFromIpfsHost(host, rt)
	return bitswap.New(lifecycleCtx(mctx, lc), bitswapNetwork, bs)
}

func onlineNamesysCtor(rt routing.IpfsRouting, repo repo.Repo, cfg *iconfig.Config) (namesys.NameSystem, error) {
	cs := cfg.Ipns.ResolveCacheSize
	if cs == 0 {
		cs = DefaultIpnsCacheSize
	}
	if cs < 0 {
		return nil, fmt.Errorf("cannot specify negative resolve cache size")
	}
	return namesys.NewNameSystem(rt, repo.Datastore(), cs), nil
}

func ipnsRepublisher(lc lcProcess, cfg *iconfig.Config, namesys namesys.NameSystem, repo repo.Repo, privKey ic.PrivKey) error {
	repub := ipnsrp.NewRepublisher(namesys, repo.Datastore(), privKey, repo.Keystore())

	if cfg.Ipns.RepublishPeriod != "" {
		d, err := time.ParseDuration(cfg.Ipns.RepublishPeriod)
		if err != nil {
			return fmt.Errorf("failure to parse config setting IPNS.RepublishPeriod: %s", err)
		}

		if !u.Debug && (d < time.Minute || d > (time.Hour*24)) {
			return fmt.Errorf("config setting IPNS.RepublishPeriod is not between 1min and 1day: %s", d)
		}

		repub.Interval = d
	}

	if cfg.Ipns.RecordLifetime != "" {
		d, err := time.ParseDuration(cfg.Ipns.RecordLifetime)
		if err != nil {
			return fmt.Errorf("failure to parse config setting IPNS.RecordLifetime: %s", err)
		}

		repub.RecordLifetime = d
	}

	lc.Run(repub.Run)
	return nil
}

type discoveryHandler struct {
	ctx  context.Context
	host p2phost.Host
}

func (dh *discoveryHandler) HandlePeerFound(p pstore.PeerInfo) {
	log.Warning("trying peer info: ", p)
	ctx, cancel := context.WithTimeout(dh.ctx, discoveryConnTimeout)
	defer cancel()
	if err := dh.host.Connect(ctx, p); err != nil {
		log.Warning("Failed to connect to peer found by discovery: ", err)
	}
}

func newDiscoveryHandler(mctx MetricsCtx, lc fx.Lifecycle, host p2phost.Host) *discoveryHandler {
	return &discoveryHandler{
		ctx:  lifecycleCtx(mctx, lc),
		host: host,
	}
}

func setupDiscovery(mctx MetricsCtx, lc fx.Lifecycle, cfg *iconfig.Config, host p2phost.Host, handler *discoveryHandler) error {
	if cfg.Discovery.MDNS.Enabled {
		mdns := cfg.Discovery.MDNS
		if mdns.Interval == 0 {
			mdns.Interval = 5
		}
		service, err := discovery.NewMdnsService(lifecycleCtx(mctx, lc), host, time.Duration(mdns.Interval)*time.Second, discovery.ServiceTag)
		if err != nil {
			log.Error("mdns error: ", err)
			return nil
		}
		service.RegisterNotifee(handler)
	}
	return nil
}

func providerQueue(mctx MetricsCtx, lc fx.Lifecycle, repo repo.Repo) (*provider.Queue, error) {
	return provider.NewQueue(lifecycleCtx(mctx, lc), "provider-v1", repo.Datastore())
}

func providerCtor(mctx MetricsCtx, lc fx.Lifecycle, queue *provider.Queue, rt routing.IpfsRouting) provider.Provider {
	return provider.NewProvider(lifecycleCtx(mctx, lc), queue, rt)
}

func reproviderCtor(mctx MetricsCtx, lc fx.Lifecycle, cfg *iconfig.Config, bs BaseBlocks, ds format.DAGService, pinning pin.Pinner, rt routing.IpfsRouting) (*rp.Reprovider, error) {
	var keyProvider rp.KeyChanFunc

	switch cfg.Reprovider.Strategy {
	case "all":
		fallthrough
	case "":
		keyProvider = rp.NewBlockstoreProvider(bs)
	case "roots":
		keyProvider = rp.NewPinnedProvider(pinning, ds, true)
	case "pinned":
		keyProvider = rp.NewPinnedProvider(pinning, ds, false)
	default:
		return nil, fmt.Errorf("unknown reprovider strategy '%s'", cfg.Reprovider.Strategy)
	}
	return rp.NewReprovider(lifecycleCtx(mctx, lc), rt, keyProvider), nil
}

func reprovider(cfg *iconfig.Config, reprovider *rp.Reprovider) error {
	reproviderInterval := kReprovideFrequency
	if cfg.Reprovider.Interval != "" {
		dur, err := time.ParseDuration(cfg.Reprovider.Interval)
		if err != nil {
			return err
		}

		reproviderInterval = dur
	}

	go reprovider.Run(reproviderInterval)
	return nil
}

func files(mctx MetricsCtx, lc fx.Lifecycle, repo repo.Repo, dag format.DAGService) (*mfs.Root, error) {
	dsk := ds.NewKey("/local/filesroot")
	pf := func(ctx context.Context, c cid.Cid) error {
		return repo.Datastore().Put(dsk, c.Bytes())
	}

	var nd *merkledag.ProtoNode
	val, err := repo.Datastore().Get(dsk)
	ctx := lifecycleCtx(mctx, lc)

	switch {
	case err == ds.ErrNotFound || val == nil:
		nd = ft.EmptyDirNode()
		err := dag.Add(ctx, nd)
		if err != nil {
			return nil, fmt.Errorf("failure writing to dagstore: %s", err)
		}
	case err == nil:
		c, err := cid.Cast(val)
		if err != nil {
			return nil, err
		}

		rnd, err := dag.Get(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("error loading filesroot from DAG: %s", err)
		}

		pbnd, ok := rnd.(*merkledag.ProtoNode)
		if !ok {
			return nil, merkledag.ErrNotProtobuf
		}

		nd = pbnd
	default:
		return nil, err
	}

	return mfs.NewRoot(ctx, dag, nd, pf)
}

////////////
// Hacks

// lifecycleCtx creates a context which will be cancelled when lifecycle stops
//
// This is a hack which we need because most of our services use contexts in a
// wrong way
func lifecycleCtx(mctx MetricsCtx, lc fx.Lifecycle) context.Context {
	ctx, cancel := context.WithCancel(mctx)
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			cancel()
			return nil
		},
	})
	return ctx
}

type lcProcess struct {
	fx.In

	LC   fx.Lifecycle
	Proc goprocess.Process
}

func (lp *lcProcess) Run(f goprocess.ProcessFunc) {
	proc := make(chan goprocess.Process, 1)
	lp.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			proc <- lp.Proc.Go(f)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return (<-proc).Close() // todo: respect ctx, somehow
		},
	})
}

func baseProcess(lc fx.Lifecycle) goprocess.Process {
	p := goprocess.WithParent(goprocess.Background())
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return p.Close()
		},
	})
	return p
}

func setupSharding(cfg *iconfig.Config) {
	// TEMP: setting global sharding switch here
	uio.UseHAMTSharding = cfg.Experimental.ShardingEnabled
}
