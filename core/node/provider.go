package node

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/fetcher"
	pin "github.com/ipfs/boxo/pinning/pinner"
	provider "github.com/ipfs/boxo/provider"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
	"go.uber.org/fx"
)

// The size of a batch that will be used for calculating average announcement
// time per CID, inside of boxo/provider.ThroughputReport
// and in 'ipfs stats provide' report.
const sampledBatchSize = 1000

func ProviderSys(reprovideInterval time.Duration, acceleratedDHTClient bool) fx.Option {
	return fx.Provide(func(lc fx.Lifecycle, cr irouting.ProvideManyRouter, keyProvider provider.KeyChanFunc, repo repo.Repo, bs blockstore.Blockstore) (provider.System, error) {
		opts := []provider.Option{
			provider.Online(cr),
			provider.ReproviderInterval(reprovideInterval),
			provider.KeyProvider(keyProvider),
		}
		if !acceleratedDHTClient {
			// The estimation kinda suck if you are running with accelerated DHT client,
			// given this message is just trying to push people to use the acceleratedDHTClient
			// let's not report on through if it's in use
			opts = append(opts,
				provider.ThroughputReport(func(reprovide bool, complete bool, keysProvided uint, duration time.Duration) bool {
					avgProvideSpeed := duration / time.Duration(keysProvided)
					count := uint64(keysProvided)

					if !reprovide || !complete {
						// We don't know how many CIDs we have to provide, try to fetch it from the blockstore.
						// But don't try for too long as this might be very expensive if you have a huge datastore.
						ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
						defer cancel()

						// FIXME: I want a running counter of blocks so size of blockstore can be an O(1) lookup.
						ch, err := bs.AllKeysChan(ctx)
						if err != nil {
							logger.Errorf("fetching AllKeysChain in provider ThroughputReport: %v", err)
							return false
						}
						count = 0
					countLoop:
						for {
							select {
							case _, ok := <-ch:
								if !ok {
									break countLoop
								}
								count++
							case <-ctx.Done():
								// really big blockstore mode

								// how many blocks would be in a 10TiB blockstore with 128KiB blocks.
								const probableBigBlockstore = (10 * 1024 * 1024 * 1024 * 1024) / (128 * 1024)
								// How long per block that lasts us.
								expectedProvideSpeed := reprovideInterval / probableBigBlockstore
								if avgProvideSpeed > expectedProvideSpeed {
									logger.Errorf(`
🔔🔔🔔 YOU MAY BE FALLING BEHIND DHT REPROVIDES! 🔔🔔🔔

⚠️ Your system might be struggling to keep up with DHT reprovides!
This means your content could partially or completely inaccessible on the network.
We observed that you recently provided %d keys at an average rate of %v per key.

🕑 An attempt to estimate your blockstore size timed out after 5 minutes,
implying your blockstore might be exceedingly large. Assuming a considerable
size of 10TiB, it would take %v to provide the complete set.

⏰ The total provide time needs to stay under your reprovide interval (%v) to prevent falling behind!

💡 Consider enabling the Accelerated DHT to enhance your system performance. See:
https://github.com/ipfs/kubo/blob/master/docs/config.md#routingaccelerateddhtclient`,
										keysProvided, avgProvideSpeed, avgProvideSpeed*probableBigBlockstore, reprovideInterval)
									return false
								}
							}
						}
					}

					// How long per block that lasts us.
					expectedProvideSpeed := reprovideInterval
					if count > 0 {
						expectedProvideSpeed = reprovideInterval / time.Duration(count)
					}

					if avgProvideSpeed > expectedProvideSpeed {
						logger.Errorf(`
🔔🔔🔔 YOU ARE FALLING BEHIND DHT REPROVIDES! 🔔🔔🔔

⚠️ Your system is struggling to keep up with DHT reprovides!
This means your content could partially or completely inaccessible on the network.
We observed that you recently provided %d keys at an average rate of %v per key.

💾 Your total CID count is ~%d which would total at %v reprovide process.

⏰ The total provide time needs to stay under your reprovide interval (%v) to prevent falling behind!

💡 Consider enabling the Accelerated DHT to enhance your reprovide throughput. See:
https://github.com/ipfs/kubo/blob/master/docs/config.md#routingaccelerateddhtclient`,
							keysProvided, avgProvideSpeed, count, avgProvideSpeed*time.Duration(count), reprovideInterval)
					}
					return false
				}, sampledBatchSize))
		}
		sys, err := provider.New(repo.Datastore(), opts...)
		if err != nil {
			return nil, err
		}

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return sys.Close()
			},
		})

		return sys, nil
	})
}

// ONLINE/OFFLINE

// OnlineProviders groups units managing provider routing records online
func OnlineProviders(useStrategicProviding bool, reprovideStrategy string, reprovideInterval time.Duration, acceleratedDHTClient bool) fx.Option {
	if useStrategicProviding {
		return OfflineProviders()
	}

	var keyProvider fx.Option
	switch reprovideStrategy {
	case "all", "":
		keyProvider = fx.Provide(newProvidingStrategy(false, false))
	case "roots":
		keyProvider = fx.Provide(newProvidingStrategy(true, true))
	case "pinned":
		keyProvider = fx.Provide(newProvidingStrategy(true, false))
	case "flat":
		keyProvider = fx.Provide(provider.NewBlockstoreProvider)
	default:
		return fx.Error(fmt.Errorf("unknown reprovider strategy %q", reprovideStrategy))
	}

	return fx.Options(
		keyProvider,
		ProviderSys(reprovideInterval, acceleratedDHTClient),
	)
}

// OfflineProviders groups units managing provider routing records offline
func OfflineProviders() fx.Option {
	return fx.Provide(provider.NewNoopProvider)
}

func newProvidingStrategy(onlyPinned, onlyRoots bool) interface{} {
	type input struct {
		fx.In
		Pinner      pin.Pinner
		Blockstore  blockstore.Blockstore
		IPLDFetcher fetcher.Factory `name:"ipldFetcher"`
	}
	return func(in input) provider.KeyChanFunc {
		if onlyRoots {
			return provider.NewPinnedProvider(true, in.Pinner, in.IPLDFetcher)
		}

		if onlyPinned {
			return provider.NewPinnedProvider(false, in.Pinner, in.IPLDFetcher)
		}

		return provider.NewPrioritizedProvider(
			provider.NewPinnedProvider(true, in.Pinner, in.IPLDFetcher),
			provider.NewBlockstoreProvider(in.Blockstore),
		)
	}
}
