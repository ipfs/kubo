package node

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/fetcher"
	"github.com/ipfs/boxo/mfs"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/boxo/provider"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
	"go.uber.org/fx"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p-kad-dht/amino"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	ddhtprovider "github.com/libp2p/go-libp2p-kad-dht/dual/provider"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	dhtprovider "github.com/libp2p/go-libp2p-kad-dht/provider"
	rds "github.com/libp2p/go-libp2p-kad-dht/provider/datastore"
	"github.com/libp2p/go-libp2p/core/routing"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
)

// The size of a batch that will be used for calculating average announcement
// time per CID, inside of boxo/provider.ThroughputReport
// and in 'ipfs stats provide' report.
const sampledBatchSize = 1000

type NoopProvider struct{}

func (r *NoopProvider) StartProviding(...mh.Multihash)                        {}
func (r *NoopProvider) StopProviding(...mh.Multihash)                         {}
func (r *NoopProvider) InstantProvide(context.Context, ...mh.Multihash) error { return nil }
func (r *NoopProvider) ForceProvide(context.Context, ...mh.Multihash) error   { return nil }

type Provider interface {
	// StartProviding provides the given keys to the DHT swarm unless they were
	// already provided in the past. The keys will be periodically reprovided until
	// StopProviding is called for the same keys or user defined garbage collection
	// deletes the keys.
	StartProviding(...mh.Multihash)

	// StopProviding stops reproviding the given keys to the DHT swarm. The node
	// stops being referred as a provider when the provider records in the DHT
	// swarm expire.
	StopProviding(...mh.Multihash)

	// InstantProvide only sends provider records for the given keys out to the DHT
	// swarm. It does NOT take the responsibility to reprovide these keys.
	InstantProvide(context.Context, ...mh.Multihash) error

	// ForceProvide is similar to StartProviding, but it sends provider records out
	// to the DHT even if the keys were already provided in the past.
	ForceProvide(context.Context, ...mh.Multihash) error
}

var (
	_ Provider = &ddhtprovider.SweepingProvider{}
	_ Provider = &dhtprovider.SweepingProvider{}
	_ Provider = &NoopProvider{}
	_ Provider = &BurstReprovider{}
)

// BurstReprovider is a wrapper around the boxo/provider.System. This DHT
// provide system manages reprovides by bursts where it sequentially reprovides
// all keys.
type BurstReprovider struct {
	provider.System
}

// StartProviding doesn't keep track of which keys have been provided so far.
// It simply calls InstantProvide to provide the given keys to the network, and
// returns instantly.
func (r *BurstReprovider) StartProviding(keys ...mh.Multihash) {
	go r.InstantProvide(context.Background(), keys...)
}

// StopProviding is a no op, since reprovider isn't tracking the keys to be
// reprovided over time.
func (r *BurstReprovider) StopProviding(keys ...mh.Multihash) {
}

// InstantProvide provides the given keys to the network without waiting.
//
// If an error is returned by the Provide operation, don't try to provide the
// remaining keys, and return the error.
func (r *BurstReprovider) InstantProvide(ctx context.Context, keys ...mh.Multihash) error {
	for _, k := range keys {
		err := r.Provide(ctx, cid.NewCidV1(cid.Raw, k), true)
		if err != nil {
			return err
		}
	}
	return nil
}

// ForceProvide is an alias for InstantProvide, it provides the given keys to
// the network, but doesn't track which keys should be reprovided since
// reprovider doesn't hold such a state.
func (r *BurstReprovider) ForceProvide(ctx context.Context, keys ...mh.Multihash) error {
	return r.InstantProvide(ctx, keys...)
}

// BurstProvider creates a BurstReprovider to be used as provider in the
// IpfsNode
func BurstReproviderOpt(reprovideInterval time.Duration, acceleratedDHTClient bool, provideWorkerCount int) fx.Option {
	return fx.Provide(func(lc fx.Lifecycle, cr irouting.ProvideManyRouter, keyProvider provider.KeyChanFunc, repo repo.Repo, bs blockstore.Blockstore) (Provider, error) {
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

		prov := &BurstReprovider{sys}

		return prov, nil
	})
}

func SweepingProvider(cfg *config.Config) fx.Option {
	mhStore := fx.Provide(func(keyProvider provider.KeyChanFunc, repo repo.Repo) (*rds.MHStore, error) {
		mhStore, err := rds.NewMHStore(context.Background(), repo.Datastore(),
			rds.WithPrefixLen(10),
			rds.WithDatastorePrefix("/reprovider/mhs"),
			rds.WithGCInterval(cfg.Reprovider.Sweep.MHStoreGCInterval.WithDefault(cfg.Reprovider.Interval.WithDefault(config.DefaultReproviderInterval))),
			rds.WithGCBatchSize(int(cfg.Reprovider.Sweep.MHStoreBatchSize.WithDefault(config.DefaultReproviderSweepMHStoreBatchSize))),
			rds.WithGCFunc(keyProvider),
		)
		if err != nil {
			return nil, err
		}
		keysChan, err := keyProvider(context.Background())
		if err != nil {
			return nil, err
		}
		err = mhStore.ResetCids(context.Background(), keysChan)
		if err != nil {
			return nil, err
		}
		return mhStore, nil
	})

	type input struct {
		fx.In
		DHT     routing.Routing `name:"dhtc"`
		MHStore *rds.MHStore
	}
	sweepingReprovider := fx.Provide(func(in input) (Provider, error) {
		switch dht := in.DHT.(type) {
		case *dual.DHT:
			if dht != nil {
				return ddhtprovider.NewSweepingProvider(dht,
					ddhtprovider.WithMHStore(in.MHStore),

					ddhtprovider.WithReprovideInterval(cfg.Reprovider.Interval.WithDefault(config.DefaultReproviderInterval)),
					ddhtprovider.WithMaxReprovideDelay(time.Hour),

					ddhtprovider.WithMaxWorkers(int(cfg.Reprovider.Sweep.MaxWorkers.WithDefault(config.DefaultReproviderSweepMaxWorkers))),
					ddhtprovider.WithDedicatedPeriodicWorkers(int(cfg.Reprovider.Sweep.DedicatedPeriodicWorkers.WithDefault(config.DefaultReproviderSweepDedicatedPeriodicWorkers))),
					ddhtprovider.WithDedicatedPeriodicWorkers(int(cfg.Reprovider.Sweep.DedicatedBurstWorkers.WithDefault(config.DefaultReproviderSweepDedicatedBurstWorkers))),
					ddhtprovider.WithMaxProvideConnsPerWorker(int(cfg.Reprovider.Sweep.MaxProvideConnsPerWorker.WithDefault(config.DefaultReproviderSweepMaxProvideConnsPerWorker))),
				)
			}
		case *fullrt.FullRT:
			if dht != nil {
				return dhtprovider.NewProvider(context.Background(),
					dhtprovider.WithMHStore(in.MHStore),

					dhtprovider.WithRouter(dht),
					dhtprovider.WithMessageSender(dht.MessageSender()),
					dhtprovider.WithPeerID(dht.Host().ID()),
					dhtprovider.WithSelfAddrs(func() []ma.Multiaddr { return dht.Host().Addrs() }),
					dhtprovider.WithAddLocalRecord(func(h mh.Multihash) error {
						return dht.Provide(context.Background(), cid.NewCidV1(cid.Raw, h), false)
					}),

					dhtprovider.WithReplicationFactor(amino.DefaultBucketSize),
					dhtprovider.WithReprovideInterval(cfg.Reprovider.Interval.WithDefault(config.DefaultReproviderInterval)),
					dhtprovider.WithMaxReprovideDelay(time.Hour),
					dhtprovider.WithConnectivityCheckOnlineInterval(1*time.Minute),
					dhtprovider.WithConnectivityCheckOfflineInterval(5*time.Minute),

					dhtprovider.WithMaxWorkers(int(cfg.Reprovider.Sweep.MaxWorkers.WithDefault(config.DefaultReproviderSweepMaxWorkers))),
					dhtprovider.WithDedicatedPeriodicWorkers(int(cfg.Reprovider.Sweep.DedicatedPeriodicWorkers.WithDefault(config.DefaultReproviderSweepDedicatedPeriodicWorkers))),
					dhtprovider.WithDedicatedPeriodicWorkers(int(cfg.Reprovider.Sweep.DedicatedBurstWorkers.WithDefault(config.DefaultReproviderSweepDedicatedBurstWorkers))),
					dhtprovider.WithMaxProvideConnsPerWorker(int(cfg.Reprovider.Sweep.MaxProvideConnsPerWorker.WithDefault(config.DefaultReproviderSweepMaxProvideConnsPerWorker))),
				)
			}
		}
		return &NoopProvider{}, nil
	})

	closeMHStore := fx.Invoke(func(lc fx.Lifecycle, mhStore *rds.MHStore) {
		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				return mhStore.Close()
			},
		})
	})

	return fx.Options(
		mhStore,
		sweepingReprovider,
		closeMHStore,
	)
}

// ONLINE/OFFLINE

// OnlineProviders groups units managing provider routing records online
func OnlineProviders(provide bool, cfg *config.Config) fx.Option {
	if !provide {
		return OfflineProviders()
	}

	reprovideStrategy := cfg.Reprovider.Strategy.WithDefault(config.DefaultReproviderStrategy)

	var keyProvider fx.Option
	switch reprovideStrategy {
	case "all", "", "roots", "pinned", "mfs", "pinned+mfs", "flat":
		keyProvider = fx.Provide(newProvidingStrategy(reprovideStrategy))
	default:
		return fx.Error(fmt.Errorf("unknown reprovider strategy %q", reprovideStrategy))
	}

	opts := []fx.Option{keyProvider}
	if cfg.Reprovider.Sweep.Enabled.WithDefault(config.DefaultReproviderSweepEnabled) {
		reprovideInterval := cfg.Reprovider.Interval.WithDefault(config.DefaultReproviderInterval)
		acceleratedDHTClient := cfg.Routing.AcceleratedDHTClient.WithDefault(config.DefaultAcceleratedDHTClient)
		provideWorkerCount := int(cfg.Provider.WorkerCount.WithDefault(config.DefaultProviderWorkerCount))

		opts = append(opts, BurstReproviderOpt(reprovideInterval, acceleratedDHTClient, provideWorkerCount))
	} else {
		opts = append(opts, SweepingProvider(cfg))
	}

	return fx.Options(opts...)
}

// OfflineProviders groups units managing provider routing records offline
func OfflineProviders() fx.Option {
	return fx.Provide(&NoopProvider{})
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
