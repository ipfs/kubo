package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"syscall"
	"time"

	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/p2p"
	"github.com/ipfs/go-ipfs/provider"

	filestore "github.com/ipfs/go-ipfs/filestore"
	namesys "github.com/ipfs/go-ipfs/namesys"
	pin "github.com/ipfs/go-ipfs/pin"
	repo "github.com/ipfs/go-ipfs/repo"
	cidv0v1 "github.com/ipfs/go-ipfs/thirdparty/cidv0v1"
	"github.com/ipfs/go-ipfs/thirdparty/verifbs"

	bserv "github.com/ipfs/go-blockservice"
	ds "github.com/ipfs/go-datastore"
	retry "github.com/ipfs/go-datastore/retrystore"
	dsync "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cfg "github.com/ipfs/go-ipfs-config"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	dag "github.com/ipfs/go-merkledag"
	metrics "github.com/ipfs/go-metrics-interface"
	resolver "github.com/ipfs/go-path/resolver"
	uio "github.com/ipfs/go-unixfs/io"
	libp2p "github.com/libp2p/go-libp2p"
	ci "github.com/libp2p/go-libp2p-crypto"
	p2phost "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

type BuildCfg struct {
	// If online is set, the node will have networking enabled
	Online bool

	// ExtraOpts is a map of extra options used to configure the ipfs nodes creation
	ExtraOpts map[string]bool

	// If permanent then node should run more expensive processes
	// that will improve performance in long run
	Permanent bool

	// DisableEncryptedConnections disables connection encryption *entirely*.
	// DO NOT SET THIS UNLESS YOU'RE TESTING.
	DisableEncryptedConnections bool

	// If NilRepo is set, a Repo backed by a nil datastore will be constructed
	NilRepo bool

	Routing RoutingOption
	Host    HostOption
	Repo    repo.Repo
}

func (cfg *BuildCfg) getOpt(key string) bool {
	if cfg.ExtraOpts == nil {
		return false
	}

	return cfg.ExtraOpts[key]
}

func (cfg *BuildCfg) fillDefaults() error {
	if cfg.Repo != nil && cfg.NilRepo {
		return errors.New("cannot set a Repo and specify nilrepo at the same time")
	}

	if cfg.Repo == nil {
		var d ds.Datastore
		if cfg.NilRepo {
			d = ds.NewNullDatastore()
		} else {
			d = ds.NewMapDatastore()
		}
		r, err := defaultRepo(dsync.MutexWrap(d))
		if err != nil {
			return err
		}
		cfg.Repo = r
	}

	if cfg.Routing == nil {
		cfg.Routing = DHTOption
	}

	if cfg.Host == nil {
		cfg.Host = DefaultHostOption
	}

	return nil
}

func defaultRepo(dstore repo.Datastore) (repo.Repo, error) {
	c := cfg.Config{}
	priv, pub, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, rand.Reader)
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	privkeyb, err := priv.Bytes()
	if err != nil {
		return nil, err
	}

	c.Bootstrap = cfg.DefaultBootstrapAddresses
	c.Addresses.Swarm = []string{"/ip4/0.0.0.0/tcp/4001"}
	c.Identity.PeerID = pid.Pretty()
	c.Identity.PrivKey = base64.StdEncoding.EncodeToString(privkeyb)

	return &repo.Mock{
		D: dstore,
		C: c,
	}, nil
}

type MetricsCtx context.Context

// NewNode constructs and returns an IpfsNode using the given cfg.
func NewNode(ctx context.Context, cfg *BuildCfg) (*IpfsNode, error) {
	if cfg == nil {
		cfg = new(BuildCfg)
	}

	err := cfg.fillDefaults()
	if err != nil {
		return nil, err
	}

	ctx = metrics.CtxScope(ctx, "ipfs")

	repoOption := fx.Provide(func(lc fx.Lifecycle) repo.Repo {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return cfg.Repo.Close()
			},
		})

		return cfg.Repo
	})

	// TODO: Remove this, use only for passing node config
	cfgOption := fx.Provide(func() *BuildCfg {
		return cfg
	})

	metricsCtx := fx.Provide(func() MetricsCtx {
		return MetricsCtx(ctx)
	})

	params := fx.Options(
		repoOption,
		cfgOption,
		metricsCtx,
	)

	storage := fx.Options(
		fx.Provide(repoConfig),
		fx.Provide(datastoreCtor),
		fx.Provide(baseBlockstoreCtor),
		fx.Provide(gcBlockstoreCtor),
	)

	ident := fx.Options(
		fx.Provide(identity),
		fx.Provide(privateKey),
		fx.Provide(peerstore),
	)

	ipns := fx.Options(
		fx.Provide(recordValidator),
	)

	providers := fx.Options(
		fx.Provide(providerQueue),
		fx.Provide(providerCtor),
		fx.Provide(reproviderCtor),

		fx.Invoke(reprovider),
		fx.Invoke(provider.Provider.Run),
	)

	online := fx.Options(
		fx.Provide(onlineExchangeCtor),
		fx.Provide(onlineNamesysCtor),

		fx.Invoke(ipnsRepublisher),

		fx.Provide(p2p.NewP2P),

		ipfsp2p,
		providers,
	)
	if !cfg.Online {
		online = fx.Options(
			fx.Provide(offline.Exchange),
			fx.Provide(offlineNamesysCtor),
			fx.Provide(offroute.NewOfflineRouter),
			fx.Provide(provider.NewOfflineProvider),
		)
	}

	core := fx.Options(
		fx.Provide(blockServiceCtor),
		fx.Provide(dagCtor),
		fx.Provide(resolver.NewBasicResolver),
		fx.Provide(pinning),
		fx.Provide(files),
	)

	n := &IpfsNode{
		ctx: ctx,
	}

	app := fx.New(
		fx.Provide(baseProcess),

		params,
		storage,
		ident,
		ipns,
		online,

		fx.Invoke(setupSharding),
		fx.NopLogger,

		core,

		fx.Extract(n),
	)

	go func() {
		<-ctx.Done()
		app.Stop(context.Background())
	}()

	n.IsOnline = cfg.Online
	n.app = app

	/*	n := &IpfsNode{
			IsOnline:  cfg.Online,
			Repo:      cfg.Repo,
			ctx:       ctx,
			Peerstore: pstoremem.NewPeerstore(),
		}

		n.RecordValidator = record.NamespacedValidator{
			"pk":   record.PublicKeyValidator{},
			"ipns": ipns.Validator{KeyBook: n.Peerstore},
		}
	*/
	// TODO: port to lifetimes
	// n.proc = goprocessctx.WithContextAndTeardown(ctx, n.teardown)

	/*if err := setupNode(ctx, n, cfg); err != nil {
		n.Close()
		return nil, err
	}*/
	if app.Err() != nil {
		return nil, app.Err()
	}

	if err := app.Start(ctx); err != nil {
		return nil, err
	}

	// TODO: DI-ify bootstrap
	if !cfg.Online {
		return n, nil
	}

	return n, n.Bootstrap(DefaultBootstrapConfig)
}

func isTooManyFDError(err error) bool {
	perr, ok := err.(*os.PathError)
	if ok && perr.Err == syscall.EMFILE {
		return true
	}

	return false
}

func setupNode(ctx context.Context, n *IpfsNode, cfg *BuildCfg) error {
	// setup local identity
	if err := n.loadID(); err != nil {
		return err
	}

	// load the private key (if present)
	if err := n.loadPrivateKey(); err != nil {
		return err
	}

	rds := &retry.Datastore{
		Batching:    n.Repo.Datastore(),
		Delay:       time.Millisecond * 200,
		Retries:     6,
		TempErrFunc: isTooManyFDError,
	}

	// hash security
	bs := bstore.NewBlockstore(rds)
	bs = &verifbs.VerifBS{Blockstore: bs}

	opts := bstore.DefaultCacheOpts()
	conf, err := n.Repo.Config()
	if err != nil {
		return err
	}

	// TEMP: setting global sharding switch here
	uio.UseHAMTSharding = conf.Experimental.ShardingEnabled

	opts.HasBloomFilterSize = conf.Datastore.BloomFilterSize
	if !cfg.Permanent {
		opts.HasBloomFilterSize = 0
	}

	if !cfg.NilRepo {
		bs, err = bstore.CachedBlockstore(ctx, bs, opts)
		if err != nil {
			return err
		}
	}

	bs = bstore.NewIdStore(bs)

	bs = cidv0v1.NewBlockstore(bs)

	n.BaseBlocks = bs
	n.GCLocker = bstore.NewGCLocker()
	n.Blockstore = bstore.NewGCBlockstore(bs, n.GCLocker)

	if conf.Experimental.FilestoreEnabled || conf.Experimental.UrlstoreEnabled {
		// hash security
		n.Filestore = filestore.NewFilestore(bs, n.Repo.FileManager())
		n.Blockstore = bstore.NewGCBlockstore(n.Filestore, n.GCLocker)
		n.Blockstore = &verifbs.VerifBSGC{GCBlockstore: n.Blockstore}
	}

	rcfg, err := n.Repo.Config()
	if err != nil {
		return err
	}

	if rcfg.Datastore.HashOnRead {
		bs.HashOnRead(true)
	}

	hostOption := cfg.Host
	if cfg.DisableEncryptedConnections {
		innerHostOption := hostOption
		hostOption = func(ctx context.Context, id peer.ID, ps pstore.Peerstore, options ...libp2p.Option) (p2phost.Host, error) {
			return innerHostOption(ctx, id, ps, append(options, libp2p.NoSecurity)...)
		}
		// TODO: shouldn't this be Errorf to guarantee visibility?
		log.Warningf(`Your IPFS node has been configured to run WITHOUT ENCRYPTED CONNECTIONS.
		You will not be able to connect to any nodes configured to use encrypted connections`)
	}

	if cfg.Online {
		do := setupDiscoveryOption(rcfg.Discovery)
		if err := n.startOnlineServices(ctx, cfg.Routing, hostOption, do, cfg.getOpt("pubsub"), cfg.getOpt("ipnsps"), cfg.getOpt("mplex")); err != nil {
			return err
		}
	} else {
		n.Exchange = offline.Exchange(n.Blockstore)
		n.Routing = offroute.NewOfflineRouter(n.Repo.Datastore(), n.RecordValidator)
		n.Namesys = namesys.NewNameSystem(n.Routing, n.Repo.Datastore(), 0)
	}

	n.Blocks = bserv.New(n.Blockstore, n.Exchange)
	n.DAG = dag.NewDAGService(n.Blocks)

	internalDag := dag.NewDAGService(bserv.New(n.Blockstore, offline.Exchange(n.Blockstore)))
	n.Pinning, err = pin.LoadPinner(n.Repo.Datastore(), n.DAG, internalDag)
	if err != nil {
		// TODO: we should move towards only running 'NewPinner' explicitly on
		// node init instead of implicitly here as a result of the pinner keys
		// not being found in the datastore.
		// this is kinda sketchy and could cause data loss
		n.Pinning = pin.NewPinner(n.Repo.Datastore(), n.DAG, internalDag)
	}
	n.Resolver = resolver.NewBasicResolver(n.DAG)

	// Provider
	queue, err := provider.NewQueue(ctx, "provider-v1", n.Repo.Datastore())
	if err != nil {
		return err
	}
	n.Provider = provider.NewProvider(ctx, queue, n.Routing)

	if cfg.Online {
		if err := n.startLateOnlineServices(ctx); err != nil {
			return err
		}
	}

	return n.loadFilesRoot()
}
