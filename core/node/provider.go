package node

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-fetcher"
	pin "github.com/ipfs/go-ipfs-pinner"
	provider "github.com/ipfs/go-ipfs-provider"
	"github.com/ipfs/go-ipfs-provider/batched"
	q "github.com/ipfs/go-ipfs-provider/queue"
	"github.com/ipfs/go-ipfs-provider/simple"
	"go.uber.org/fx"

	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
)

// SIMPLE

// ProviderQueue creates new datastore backed provider queue
func ProviderQueue(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo) (*q.Queue, error) {
	return q.NewQueue(helpers.LifecycleCtx(mctx, lc), "provider-v1", repo.Datastore())
}

// SimpleProvider creates new record provider
func SimpleProvider(mctx helpers.MetricsCtx, lc fx.Lifecycle, queue *q.Queue, rt irouting.ProvideManyRouter) provider.Provider {
	return simple.NewProvider(helpers.LifecycleCtx(mctx, lc), queue, rt)
}

// SimpleReprovider creates new reprovider
func SimpleReprovider(reproviderInterval time.Duration) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, rt irouting.ProvideManyRouter, keyProvider simple.KeyChanFunc) (provider.Reprovider, error) {
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

// BatchedProviderSys creates new provider system
func BatchedProviderSys(isOnline bool, reprovideInterval time.Duration) interface{} {
	return func(lc fx.Lifecycle, cr irouting.ProvideManyRouter, q *q.Queue, keyProvider simple.KeyChanFunc, repo repo.Repo) (provider.System, error) {
		sys, err := batched.New(cr, q,
			batched.ReproviderInterval(reprovideInterval),
			batched.Datastore(repo.Datastore()),
			batched.KeyProvider(keyProvider))
		if err != nil {
			return nil, err
		}

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

		return sys, nil
	}
}

// ONLINE/OFFLINE

// OnlineProviders groups units managing provider routing records online
func OnlineProviders(useStrategicProviding bool, useBatchedProviding bool, reprovideStrategy string, reprovideInterval time.Duration) fx.Option {
	if useStrategicProviding {
		return fx.Provide(provider.NewOfflineProvider)
	}

	return fx.Options(
		SimpleProviders(reprovideStrategy, reprovideInterval),
		maybeProvide(SimpleProviderSys(true), !useBatchedProviding),
		maybeProvide(BatchedProviderSys(true, reprovideInterval), useBatchedProviding),
	)
}

// OfflineProviders groups units managing provider routing records offline
func OfflineProviders(useStrategicProviding bool, useBatchedProviding bool, reprovideStrategy string, reprovideInterval time.Duration) fx.Option {
	if useStrategicProviding {
		return fx.Provide(provider.NewOfflineProvider)
	}

	return fx.Options(
		SimpleProviders(reprovideStrategy, reprovideInterval),
		maybeProvide(SimpleProviderSys(false), true),
		//maybeProvide(BatchedProviderSys(false, reprovideInterval), useBatchedProviding),
	)
}

// SimpleProviders creates the simple provider/reprovider dependencies
func SimpleProviders(reprovideStrategy string, reproviderInterval time.Duration) fx.Option {
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
	type input struct {
		fx.In
		Pinner      pin.Pinner
		IPLDFetcher fetcher.Factory `name:"ipldFetcher"`
	}
	return func(in input) simple.KeyChanFunc {
		return simple.NewPinnedProvider(onlyRoots, in.Pinner, in.IPLDFetcher)
	}
}
