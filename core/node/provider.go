package node

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-ipfs-pinner"
	"github.com/ipfs/go-ipfs-provider"
	q "github.com/ipfs/go-ipfs-provider/queue"
	"github.com/ipfs/go-ipfs-provider/simple"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/routing"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/repo"
)

const kReprovideFrequency = time.Hour * 12

// SIMPLE

// ProviderQueue creates new datastore backed provider queue
func ProviderQueue(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo) (*q.Queue, error) {
	return q.NewQueue(helpers.LifecycleCtx(mctx, lc), "provider-v1", repo.Datastore())
}

// SimpleProvider creates new record provider
func SimpleProvider(mctx helpers.MetricsCtx, lc fx.Lifecycle, queue *q.Queue, rt routing.Routing) provider.Provider {
	return simple.NewProvider(helpers.LifecycleCtx(mctx, lc), queue, rt)
}

// SimpleReprovider creates new reprovider
func SimpleReprovider(reproviderInterval time.Duration) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, rt routing.Routing, keyProvider simple.KeyChanFunc) (provider.Reprovider, error) {
		return simple.NewReprovider(helpers.LifecycleCtx(mctx, lc), reproviderInterval, rt, keyProvider), nil
	}
}

// SimpleProviderSys creates new provider system
func SimpleProviderSys(isOnline bool) interface{} {
	return func(lc fx.Lifecycle, p provider.Provider, r provider.Reprovider) provider.System {
		sys := provider.NewSystem(p, r)

		if isOnline {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					sys.Run()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					return sys.Close()
				},
			})
		}

		return sys
	}
}

// ONLINE/OFFLINE

// OnlineProviders groups units managing provider routing records online
func OnlineProviders(useStrategicProviding bool, reprovideStrategy string, reprovideInterval string) fx.Option {
	if useStrategicProviding {
		return fx.Provide(provider.NewOfflineProvider)
	}

	return fx.Options(
		SimpleProviders(reprovideStrategy, reprovideInterval),
		fx.Provide(SimpleProviderSys(true)),
	)
}

// OfflineProviders groups units managing provider routing records offline
func OfflineProviders(useStrategicProviding bool, reprovideStrategy string, reprovideInterval string) fx.Option {
	if useStrategicProviding {
		return fx.Provide(provider.NewOfflineProvider)
	}

	return fx.Options(
		SimpleProviders(reprovideStrategy, reprovideInterval),
		fx.Provide(SimpleProviderSys(false)),
	)
}

// SimpleProviders creates the simple provider/reprovider dependencies
func SimpleProviders(reprovideStrategy string, reprovideInterval string) fx.Option {
	reproviderInterval := kReprovideFrequency
	if reprovideInterval != "" {
		dur, err := time.ParseDuration(reprovideInterval)
		if err != nil {
			return fx.Error(err)
		}

		reproviderInterval = dur
	}

	var keyProvider fx.Option
	switch reprovideStrategy {
	case "all":
		fallthrough
	case "":
		keyProvider = fx.Provide(simple.NewBlockstoreProvider)
	case "roots":
		keyProvider = fx.Provide(pinnedProviderStrategy(true))
	case "pinned":
		keyProvider = fx.Provide(pinnedProviderStrategy(false))
	default:
		return fx.Error(fmt.Errorf("unknown reprovider strategy '%s'", reprovideStrategy))
	}

	return fx.Options(
		fx.Provide(ProviderQueue),
		fx.Provide(SimpleProvider),
		keyProvider,
		fx.Provide(SimpleReprovider(reproviderInterval)),
	)
}

func pinnedProviderStrategy(onlyRoots bool) interface{} {
	return func(pinner pin.Pinner, dag ipld.DAGService) simple.KeyChanFunc {
		return simple.NewPinnedProvider(onlyRoots, pinner, dag)
	}
}
