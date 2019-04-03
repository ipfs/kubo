package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/node"

	repo "github.com/ipfs/go-ipfs/repo"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	cfg "github.com/ipfs/go-ipfs-config"
	metrics "github.com/ipfs/go-metrics-interface"
	resolver "github.com/ipfs/go-path/resolver"
	ci "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
)

type BuildCfg node.BuildCfg

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
		cfg.Routing = node.DHTOption
	}

	if cfg.Host == nil {
		cfg.Host = node.DefaultHostOption
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

	metricsCtx := fx.Provide(func() node.MetricsCtx {
		return node.MetricsCtx(ctx)
	})

	hostOption := fx.Provide(func() node.HostOption {
		return cfg.Host
	})

	routingOption := fx.Provide(func() node.RoutingOption {
		return cfg.Routing
	})

	params := fx.Options(
		repoOption,
		hostOption,
		routingOption,
		metricsCtx,
	)

	core := fx.Options(
		fx.Provide(node.BlockServiceCtor),
		fx.Provide(node.DagCtor),
		fx.Provide(resolver.NewBasicResolver),
		fx.Provide(node.Pinning),
		fx.Provide(node.Files),
	)

	n := &IpfsNode{
		ctx: ctx,
	}

	app := fx.New(
		fx.NopLogger,
		fx.Provide(baseProcess),

		params,
		node.Storage((*node.BuildCfg)(cfg)),
		node.Identity,
		node.IPNS,
		node.Networked((*node.BuildCfg)(cfg)),

		fx.Invoke(setupSharding),

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

	// TODO: How soon will bootstrap move to libp2p?
	if !cfg.Online {
		return n, nil
	}

	return n, n.Bootstrap(bootstrap.DefaultBootstrapConfig)
}
