package node

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/fetcher"
	"github.com/ipfs/boxo/mfs"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/boxo/pinning/pinner/dspinner"
	provider "github.com/ipfs/boxo/provider"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
	"go.uber.org/fx"
)

// The size of a batch that will be used for calculating average announcement
// time per CID, inside of boxo/provider.ThroughputReport
// and in 'ipfs stats provide' report.
const sampledBatchSize = 1000

// Datastore key used to store previous reprovide strategy.
const reprovideStrategyKey = "/reprovideStrategy"

func ProviderSys(reprovideInterval time.Duration, acceleratedDHTClient bool, provideWorkerCount int) fx.Option {
	return fx.Provide(func(lc fx.Lifecycle, cr irouting.ProvideManyRouter, repo repo.Repo) (provider.System, error) {
		// Initialize provider.System first, before pinner/blockstore/etc.
		// The KeyChanFunc will be set later via SetKeyProvider() once we have
		// created the pinner, blockstore and other dependencies.
		opts := []provider.Option{
			provider.Online(cr),
			provider.ReproviderInterval(reprovideInterval),
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
						// Note: talk to datastore directly, as to not depend on Blockstore here.
						qr, err := repo.Datastore().Query(ctx, query.Query{
							Prefix:   blockstore.BlockPrefix.String(),
							KeysOnly: true})
						if err != nil {
							logger.Errorf("fetching AllKeysChain in provider ThroughputReport: %v", err)
							return false
						}
						defer qr.Close()
						count = 0
					countLoop:
						for {
							select {
							case _, ok := <-qr.Next():
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
func OnlineProviders(provide bool, providerStrategy string, reprovideInterval time.Duration, acceleratedDHTClient bool, provideWorkerCount int) fx.Option {
	if !provide {
		return OfflineProviders()
	}

	strategyFlag := config.ParseReproviderStrategy(providerStrategy)
	if strategyFlag == 0 {
		return fx.Error(fmt.Errorf("unknown reprovider strategy %q", providerStrategy))
	}

	return fx.Options(
		fx.Provide(setReproviderKeyProvider(providerStrategy)),
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

type provStrategyIn struct {
	fx.In
	Pinner               pin.Pinner
	Blockstore           blockstore.Blockstore
	OfflineIPLDFetcher   fetcher.Factory `name:"offlineIpldFetcher"`
	OfflineUnixFSFetcher fetcher.Factory `name:"offlineUnixfsFetcher"`
	MFSRoot              *mfs.Root
	Provider             provider.System
	Repo                 repo.Repo
}

type provStrategyOut struct {
	fx.Out
	ProvidingStrategy    config.ReproviderStrategy
	ProvidingKeyChanFunc provider.KeyChanFunc
}

// createKeyProvider creates the appropriate KeyChanFunc based on strategy.
// Each strategy has different behavior:
// - "roots": Only root CIDs of pinned content
// - "pinned": All pinned content (roots + children)
// - "mfs": Only MFS content
// - "flat": All blocks, no prioritization
// - "all": Prioritized: pins first, then MFS roots, then all blocks
func createKeyProvider(strategyFlag config.ReproviderStrategy, in provStrategyIn) provider.KeyChanFunc {
	switch strategyFlag {
	case config.ReproviderStrategyRoots:
		return provider.NewBufferedProvider(dspinner.NewPinnedProvider(true, in.Pinner, in.OfflineIPLDFetcher))
	case config.ReproviderStrategyPinned:
		return provider.NewBufferedProvider(dspinner.NewPinnedProvider(false, in.Pinner, in.OfflineIPLDFetcher))
	case config.ReproviderStrategyPinned | config.ReproviderStrategyMFS:
		return provider.NewPrioritizedProvider(
			provider.NewBufferedProvider(dspinner.NewPinnedProvider(false, in.Pinner, in.OfflineIPLDFetcher)),
			mfsProvider(in.MFSRoot, in.OfflineUnixFSFetcher),
		)
	case config.ReproviderStrategyMFS:
		return mfsProvider(in.MFSRoot, in.OfflineUnixFSFetcher)
	case config.ReproviderStrategyFlat:
		return in.Blockstore.AllKeysChan
	default: // "all", ""
		return createAllStrategyProvider(in)
	}
}

// createAllStrategyProvider creates the complex "all" strategy provider.
// This implements a three-tier priority system:
// 1. Root blocks of direct and recursive pins (highest priority)
// 2. MFS root (medium priority)
// 3. All other blocks in blockstore (lowest priority)
func createAllStrategyProvider(in provStrategyIn) provider.KeyChanFunc {
	return provider.NewPrioritizedProvider(
		provider.NewPrioritizedProvider(
			provider.NewBufferedProvider(dspinner.NewPinnedProvider(true, in.Pinner, in.OfflineIPLDFetcher)),
			mfsRootProvider(in.MFSRoot),
		),
		in.Blockstore.AllKeysChan,
	)
}

// detectStrategyChange checks if the reproviding strategy has changed from what's persisted.
// Returns: (previousStrategy, hasChanged, error)
func detectStrategyChange(ctx context.Context, strategy string, ds datastore.Datastore) (string, bool, error) {
	strategyKey := datastore.NewKey(reprovideStrategyKey)

	prev, err := ds.Get(ctx, strategyKey)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return "", strategy != "", nil
		}
		return "", false, err
	}

	previousStrategy := string(prev)
	return previousStrategy, previousStrategy != strategy, nil
}

// persistStrategy saves the current reproviding strategy to the datastore.
// Empty string strategies are deleted rather than stored.
func persistStrategy(ctx context.Context, strategy string, ds datastore.Datastore) error {
	strategyKey := datastore.NewKey(reprovideStrategyKey)

	if strategy == "" {
		return ds.Delete(ctx, strategyKey)
	}
	return ds.Put(ctx, strategyKey, []byte(strategy))
}

// handleStrategyChange manages strategy change detection and queue clearing.
// Strategy change detection: when the reproviding strategy changes,
// we clear the provide queue to avoid unexpected behavior from mixing
// strategies. This ensures a clean transition between different providing modes.
func handleStrategyChange(strategy string, provider provider.System, ds datastore.Datastore) {
	ctx := context.Background()

	previous, changed, err := detectStrategyChange(ctx, strategy, ds)
	if err != nil {
		logger.Error("cannot read previous reprovide strategy", "err", err)
		return
	}

	if !changed {
		return
	}

	logger.Infow("Reprovider.Strategy changed, clearing provide queue", "previous", previous, "current", strategy)
	provider.Clear()

	if err := persistStrategy(ctx, strategy, ds); err != nil {
		logger.Error("cannot update reprovide strategy", "err", err)
	}
}

func setReproviderKeyProvider(strategy string) func(in provStrategyIn) provStrategyOut {
	strategyFlag := config.ParseReproviderStrategy(strategy)

	return func(in provStrategyIn) provStrategyOut {
		// Create the appropriate key provider based on strategy
		kcf := createKeyProvider(strategyFlag, in)

		// SetKeyProvider breaks the circular dependency between provider, blockstore, and pinner.
		// We cannot create the blockstore without the provider (it needs to provide blocks),
		// and we cannot determine the reproviding strategy without the pinner/blockstore.
		// This deferred initialization allows us to create provider.System first,
		// then set the actual key provider function after all dependencies are ready.
		in.Provider.SetKeyProvider(kcf)

		// Handle strategy changes (detection, queue clearing, persistence)
		handleStrategyChange(strategy, in.Provider, in.Repo.Datastore())

		return provStrategyOut{
			ProvidingStrategy:    strategyFlag,
			ProvidingKeyChanFunc: kcf,
		}
	}
}
