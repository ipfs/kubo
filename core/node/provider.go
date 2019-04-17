package node

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/provider"
	q "github.com/ipfs/go-ipfs/provider/queue"
	"github.com/ipfs/go-ipfs/provider/simple"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/libp2p/go-libp2p-routing"
)

const kReprovideFrequency = time.Hour * 12

// SIMPLE

// ProviderQueue creates new datastore backed provider queue
func ProviderQueue(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo) (*q.Queue, error) {
	return q.NewQueue(helpers.LifecycleCtx(mctx, lc), "provider-v1", repo.Datastore())
}

// SimpleProviderCtor creates new record provider
func SimpleProviderCtor(mctx helpers.MetricsCtx, lc fx.Lifecycle, queue *q.Queue, rt routing.IpfsRouting) provider.Provider {
	return simple.NewProvider(helpers.LifecycleCtx(mctx, lc), queue, rt)
}

// SimpleReproviderCtor creates new reprovider
func SimpleReproviderCtor(reproviderInterval time.Duration) func(helpers.MetricsCtx, fx.Lifecycle, routing.IpfsRouting, simple.KeyChanFunc) (provider.Reprovider, error) {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, rt routing.IpfsRouting, keyProvider simple.KeyChanFunc) (provider.Reprovider, error) {
		return simple.NewReprovider(helpers.LifecycleCtx(mctx, lc), reproviderInterval, rt, keyProvider), nil
	}
}

// SimpleProviderSysCtor creates new provider system
func SimpleProviderSysCtor(lc fx.Lifecycle, p provider.Provider, r provider.Reprovider) provider.System {
	sys := provider.NewSystem(p, r)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			sys.Run()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return sys.Close()
		},
	})
	return sys
}

// SimpleOfflineProviderSysCtor creates a new offline provider system
func SimpleOfflineProviderSysCtor(p provider.Provider, r provider.Reprovider) provider.System {
	return provider.NewSystem(p, r)
}

// STRATEGIC

// StrategicProviderSysCtor creates new provider system
func StrategicProviderSysCtor() provider.System {
	return provider.NewOfflineProvider()
}

// StrategicOfflineProviderSysCtor creates a new offline provider system
func StrategicOfflineProviderSysCtor() provider.System {
	return provider.NewOfflineProvider()
}

// ONLINE/OFFLINE

// OnlineProviders groups units managing provider routing records online
func OnlineProviders(cfg *config.Config) fx.Option {
	if cfg.Experimental.StrategicProviding {
		return fx.Provide(StrategicProviderSysCtor)
	}

	return fx.Options(
		SimpleProviders(cfg),
		fx.Provide(SimpleProviderSysCtor),
	)
}

// OfflineProviders groups units managing provider routing records offline
func OfflineProviders(cfg *config.Config) fx.Option {
	if cfg.Experimental.StrategicProviding {
		return fx.Provide(StrategicOfflineProviderSysCtor)
	}

	return fx.Options(
		SimpleProviders(cfg),
		fx.Provide(SimpleOfflineProviderSysCtor),
	)
}

// SimpleProviders creates the simple provider/reprovider dependencies
func SimpleProviders(cfg *config.Config) fx.Option {
	reproviderInterval := kReprovideFrequency
	if cfg.Reprovider.Interval != "" {
		dur, err := time.ParseDuration(cfg.Reprovider.Interval)
		if err != nil {
			return fx.Error(err)
		}

		reproviderInterval = dur
	}

	var keyProvider fx.Option
	switch cfg.Reprovider.Strategy {
	case "all":
		fallthrough
	case "":
		keyProvider = fx.Provide(simple.NewBlockstoreProvider)
	case "roots":
		keyProvider = fx.Provide(simple.NewPinnedProvider(true))
	case "pinned":
		keyProvider = fx.Provide(simple.NewPinnedProvider(false))
	default:
		return fx.Error(fmt.Errorf("unknown reprovider strategy '%s'", cfg.Reprovider.Strategy))
	}

	return fx.Options(
		fx.Provide(ProviderQueue),
		fx.Provide(SimpleProviderCtor),
		keyProvider,
		fx.Provide(SimpleReproviderCtor(reproviderInterval)),
	)
}
