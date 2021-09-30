package node

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-fetcher"
	"github.com/ipfs/go-ipfs-pinner"
	"github.com/ipfs/go-ipfs-provider"
	"github.com/ipfs/go-ipfs-provider/batched"
	q "github.com/ipfs/go-ipfs-provider/queue"
	"github.com/ipfs/go-ipfs-provider/simple"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/multiformats/go-multihash"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
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

type provideMany interface {
	ProvideMany(ctx context.Context, keys []multihash.Multihash) error
	Ready() bool
}

// BatchedProviderSys creates new provider system
func BatchedProviderSys(isOnline bool, reprovideInterval string) interface{} {
	return func(lc fx.Lifecycle, cr libp2p.BaseIpfsRouting, q *q.Queue, keyProvider simple.KeyChanFunc, repo repo.Repo) (provider.System, error) {
		r, ok := (cr).(provideMany)
		if !ok {
			return nil, fmt.Errorf("BatchedProviderSys requires a content router that supports provideMany")
		}

		reprovideIntervalDuration := kReprovideFrequency
		if reprovideInterval != "" {
			dur, err := time.ParseDuration(reprovideInterval)
			if err != nil {
				return nil, err
			}

			reprovideIntervalDuration = dur
		}

		sys, err := batched.New(r, q,
			batched.ReproviderInterval(reprovideIntervalDuration),
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
func OnlineProviders(useStrategicProviding bool, useBatchedProviding bool, reprovideStrategy string, reprovideInterval string) fx.Option {
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
func OfflineProviders(useStrategicProviding bool, useBatchedProviding bool, reprovideStrategy string, reprovideInterval string) fx.Option {
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
	type input struct {
		fx.In
		Pinner      pin.Pinner
		IPLDFetcher fetcher.Factory `name:"ipldFetcher"`
	}
	return func(in input) simple.KeyChanFunc {
		return simple.NewPinnedProvider(onlyRoots, in.Pinner, in.IPLDFetcher)
	}
}
