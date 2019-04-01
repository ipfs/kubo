package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"syscall"

	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/p2p"
	"github.com/ipfs/go-ipfs/provider"

	repo "github.com/ipfs/go-ipfs/repo"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	cfg "github.com/ipfs/go-ipfs-config"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	metrics "github.com/ipfs/go-metrics-interface"
	resolver "github.com/ipfs/go-path/resolver"
	ci "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
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
		// Note that some services use contexts to signal shutting down, which is
		// very suboptimal. This needs to be here until that's addressed somehow
		<-ctx.Done()
		app.Stop(context.Background())
	}()

	n.IsOnline = cfg.Online
	n.app = app

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
