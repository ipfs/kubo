package newnd

import (
	"context"

	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	offroute "github.com/ipfs/go-ipfs-routing/offline"
	"github.com/ipfs/go-metrics-interface"
	"github.com/ipfs/go-path/resolver"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node"
	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/provider"
	"github.com/ipfs/go-ipfs/repo"
)

const (
	// TODO: docstrings on everything

	Goprocess component = iota

	Repo
	Config
	BatchDatastore
	Datastore
	BlockstoreBasic
	BlockstoreFinal

	Peerid
	Peerstore

	Validator
	Router
	Provider
	Exchange
	Namesys

	Blockservice
	Dag
	Pinning
	Files
	Resolver

	FxLogger

	nComponents // MUST be last
)

type component int

type settings struct {
	components []fx.Option
	userFx     []fx.Option

	ctx context.Context
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

func Override(c component, replacement interface{}) Option {
	return func(s *settings) error {
		s.components[c] = fx.Provide(replacement)

		// TODO: might do type checking with reflaction, but probably not worth it
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
	return Override(BatchDatastore, ds.Batching(ds.NewNullDatastore()))
}

func defaults() settings {
	out := settings{
		components: make([]fx.Option, nComponents),

		ctx: context.Background(),
	}

	out.components[Goprocess] = fx.Provide(baseProcess)
	out.components[Repo] = fx.Provide(memRepo)

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

func New(opts ...Option) (*coreapi.CoreAPI, error) {
	settings := defaults()
	if err := Options(opts...)(&settings); err != nil {
		return nil, err
	}

	ctx := metrics.CtxScope(settings.ctx, "ipfs")
	fxOpts := make([]fx.Option, len(settings.userFx)+len(settings.components)+2)
	for i, opt := range settings.userFx {
		fxOpts[i] = opt
	}

	for i, opt := range settings.components {
		fxOpts[i+len(settings.userFx)] = opt
	}

	n := &core.IpfsNode{}

	fxOpts[len(fxOpts)-2] = fx.Provide(func() helpers.MetricsCtx {
		return helpers.MetricsCtx(ctx)
	})
	fxOpts[len(fxOpts)-1] = fx.Extract(n)

	app := fx.New(fxOpts...)
	n.SetupCtx(ctx, app)

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

	return coreapi.NewCoreAPI(n)
}
