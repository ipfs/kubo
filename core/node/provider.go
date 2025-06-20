package node

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/fetcher"
	"github.com/ipfs/boxo/mfs"
	pin "github.com/ipfs/boxo/pinning/pinner"
	provider "github.com/ipfs/boxo/provider"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
	"go.uber.org/fx"
)

// The size of a batch that will be used for calculating average announcement
// time per CID, inside of boxo/provider.ThroughputReport
// and in 'ipfs stats provide' report.
const sampledBatchSize = 1000

func ProviderSys(reprovideInterval time.Duration, acceleratedDHTClient bool, provideWorkerCount int) fx.Option {
	return fx.Provide(func(lc fx.Lifecycle, cr irouting.ProvideManyRouter, keyProvider provider.KeyChanFunc, repo repo.Repo, bs blockstore.Blockstore) (provider.System, error) {
		opts := []provider.Option{
			provider.Online(cr),
			provider.ReproviderInterval(reprovideInterval),
			provider.KeyProvider(keyProvider),
			provider.ProvideWorkerCount(provideWorkerCount),
		}
		if !acceleratedDHTClient && reprovideInterval > 0 {
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
This means your content could be partially or completely inaccessible on the network.
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
This means your content could be partially or completely inaccessible on the network.
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
func OnlineProviders(provide bool, reprovideStrategy string, reprovideInterval time.Duration, acceleratedDHTClient bool, provideWorkerCount int) fx.Option {
	if !provide {
		return OfflineProviders()
	}

	var keyProvider fx.Option
	switch reprovideStrategy {
	case "all", "", "roots", "pinned", "mfs", "pinned+mfs", "flat":
		keyProvider = fx.Provide(newProvidingStrategy(reprovideStrategy))
	default:
		return fx.Error(fmt.Errorf("unknown reprovider strategy %q", reprovideStrategy))
	}

	return fx.Options(
		keyProvider,
		ProviderSys(reprovideInterval, acceleratedDHTClient, provideWorkerCount),
	)
}

// OfflineProviders groups units managing provider routing records offline
func OfflineProviders() fx.Option {
	return fx.Provide(provider.NewNoopProvider)
}

func mfsProvider(mfsRoot *mfs.Root, fetcher fetcher.Factory) provider.KeyChanFunc {
	return func(ctx context.Context) (<-chan cid.Cid, error) {
		err := mfsRoot.FlushMemFree(ctx)
		if err != nil {
			return nil, fmt.Errorf("error flushing mfs, cannot provide MFS: %w", err)
		}
		rootNode, err := mfsRoot.GetDirectory().GetNode()
		if err != nil {
			return nil, fmt.Errorf("error loading mfs root, cannot provide MFS: %w", err)
		}

		kcf := provider.NewDAGProvider(rootNode.Cid(), fetcher)
		return kcf(ctx)
	}
}

func mfsRootProvider(mfsRoot *mfs.Root) provider.KeyChanFunc {
	return func(ctx context.Context) (<-chan cid.Cid, error) {
		rootNode, err := mfsRoot.GetDirectory().GetNode()
		if err != nil {
			return nil, fmt.Errorf("error loading mfs root, cannot provide MFS: %w", err)
		}
		ch := make(chan cid.Cid, 1)
		ch <- rootNode.Cid()
		close(ch)
		return ch, nil
	}
}

func newProvidingStrategy(strategy string) interface{} {
	type input struct {
		fx.In
		Pinner               pin.Pinner
		Blockstore           blockstore.Blockstore
		OfflineIPLDFetcher   fetcher.Factory `name:"offlineIpldFetcher"`
		OfflineUnixFSFetcher fetcher.Factory `name:"offlineUnixfsFetcher"`
		MFSRoot              *mfs.Root
	}
	return func(in input) provider.KeyChanFunc {
		switch strategy {
		case "roots":
			return provider.NewBufferedProvider(provider.NewPinnedProvider(true, in.Pinner, in.OfflineIPLDFetcher))
		case "pinned":
			return provider.NewBufferedProvider(provider.NewPinnedProvider(false, in.Pinner, in.OfflineIPLDFetcher))
		case "pinned+mfs":
			return provider.NewPrioritizedProvider(
				provider.NewBufferedProvider(provider.NewPinnedProvider(false, in.Pinner, in.OfflineIPLDFetcher)),
				mfsProvider(in.MFSRoot, in.OfflineUnixFSFetcher),
			)
		case "mfs":
			return mfsProvider(in.MFSRoot, in.OfflineUnixFSFetcher)
		case "flat":
			return provider.NewBlockstoreProvider(in.Blockstore)
		default: // "all", ""
			return provider.NewPrioritizedProvider(
				provider.NewPrioritizedProvider(
					provider.NewBufferedProvider(provider.NewPinnedProvider(true, in.Pinner, in.OfflineIPLDFetcher)),
					mfsRootProvider(in.MFSRoot),
				),
				provider.NewBlockstoreProvider(in.Blockstore),
			)
		}
	}
}
