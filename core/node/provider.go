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
	"github.com/ipfs/boxo/provider"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/amino"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	dht_pb "github.com/libp2p/go-libp2p-kad-dht/pb"
	dhtprovider "github.com/libp2p/go-libp2p-kad-dht/provider"
	"github.com/libp2p/go-libp2p-kad-dht/provider/buffered"
	ddhtprovider "github.com/libp2p/go-libp2p-kad-dht/provider/dual"
	"github.com/libp2p/go-libp2p-kad-dht/provider/keystore"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"go.uber.org/fx"
)

const (
	// The size of a batch that will be used for calculating average announcement
	// time per CID, inside of boxo/provider.ThroughputReport
	// and in 'ipfs stats provide' report.
	// Used when Provide.DHT.SweepEnabled=false
	sampledBatchSize = 1000

	// Datastore key used to store previous reprovide strategy.
	reprovideStrategyKey = "/reprovideStrategy"

	// Datastore namespace prefix for provider data.
	providerDatastorePrefix = "provider"
	// Datastore path for the provider keystore.
	keystoreDatastorePath = "keystore"
)

// Interval between reprovide queue monitoring checks for slow reprovide alerts.
// Used when Provide.DHT.SweepEnabled=true
const reprovideAlertPollInterval = 15 * time.Minute

// Number of consecutive polling intervals with sustained queue growth before
// triggering a slow reprovide alert (3 intervals = 45 minutes).
// Used when Provide.DHT.SweepEnabled=true
const consecutiveAlertsThreshold = 3

// DHTProvider is an interface for providing keys to a DHT swarm. It holds a
// state of keys to be advertised, and is responsible for periodically
// publishing provider records for these keys to the DHT swarm before the
// records expire.
type DHTProvider interface {
	// StartProviding ensures keys are periodically advertised to the DHT swarm.
	//
	// If the `keys` aren't currently being reprovided, they are added to the
	// queue to be provided to the DHT swarm as soon as possible, and scheduled
	// to be reprovided periodically. If `force` is set to true, all keys are
	// provided to the DHT swarm, regardless of whether they were already being
	// reprovided in the past. `keys` keep being reprovided until `StopProviding`
	// is called.
	//
	// This operation is asynchronous, it returns as soon as the `keys` are added
	// to the provide queue, and provides happens asynchronously.
	//
	// Returns an error if the keys couldn't be added to the provide queue. This
	// can happen if the provider is closed or if the node is currently Offline
	// (either never bootstrapped, or disconnected since more than `OfflineDelay`).
	// The schedule and provide queue depend on the network size, hence recent
	// network connectivity is essential.
	StartProviding(force bool, keys ...mh.Multihash) error
	// ProvideOnce sends provider records for the specified keys to the DHT swarm
	// only once. It does not automatically reprovide those keys afterward.
	//
	// Add the supplied multihashes to the provide queue, and return immediately.
	// The provide operation happens asynchronously.
	//
	// Returns an error if the keys couldn't be added to the provide queue. This
	// can happen if the provider is closed or if the node is currently Offline
	// (either never bootstrapped, or disconnected since more than `OfflineDelay`).
	// The schedule and provide queue depend on the network size, hence recent
	// network connectivity is essential.
	ProvideOnce(keys ...mh.Multihash) error
	// Clear clears the all the keys from the provide queue and returns the number
	// of keys that were cleared.
	//
	// The keys are not deleted from the keystore, so they will continue to be
	// reprovided as scheduled.
	Clear() int
	// RefreshSchedule scans the Keystore for any keys that are not currently
	// scheduled for reproviding. If such keys are found, it schedules their
	// associated keyspace region to be reprovided.
	//
	// This function doesn't remove prefixes that have no keys from the schedule.
	// This is done automatically during the reprovide operation if a region has no
	// keys.
	//
	// Returns an error if the provider is closed or if the node is currently
	// Offline (either never bootstrapped, or disconnected since more than
	// `OfflineDelay`). The schedule depends on the network size, hence recent
	// network connectivity is essential.
	RefreshSchedule() error
}

var (
	_ DHTProvider = &ddhtprovider.SweepingProvider{}
	_ DHTProvider = &dhtprovider.SweepingProvider{}
	_ DHTProvider = &NoopProvider{}
	_ DHTProvider = &LegacyProvider{}
)

// NoopProvider is a no-operation provider implementation that does nothing.
// It is used when providing is disabled or when no DHT is available.
// All methods return successfully without performing any actual operations.
type NoopProvider struct{}

func (r *NoopProvider) StartProviding(bool, ...mh.Multihash) error { return nil }
func (r *NoopProvider) ProvideOnce(...mh.Multihash) error          { return nil }
func (r *NoopProvider) Clear() int                                 { return 0 }
func (r *NoopProvider) RefreshSchedule() error                     { return nil }

// LegacyProvider is a wrapper around the boxo/provider.System that implements
// the DHTProvider interface. This provider manages reprovides using a burst
// strategy where it sequentially reprovides all keys at once during each
// reprovide interval, rather than spreading the load over time.
//
// This is the legacy provider implementation that can cause resource spikes
// during reprovide operations. For more efficient providing, consider using
// the SweepingProvider which spreads the load over the reprovide interval.
type LegacyProvider struct {
	provider.System
}

func (r *LegacyProvider) StartProviding(force bool, keys ...mh.Multihash) error {
	return r.ProvideOnce(keys...)
}

func (r *LegacyProvider) ProvideOnce(keys ...mh.Multihash) error {
	if many, ok := r.System.(routinghelpers.ProvideManyRouter); ok {
		return many.ProvideMany(context.Background(), keys)
	}

	for _, k := range keys {
		if err := r.Provide(context.Background(), cid.NewCidV1(cid.Raw, k), true); err != nil {
			return err
		}
	}
	return nil
}

func (r *LegacyProvider) Clear() int {
	return r.System.Clear()
}

func (r *LegacyProvider) RefreshSchedule() error { return nil }

// LegacyProviderOpt creates a LegacyProvider to be used as provider in the
// IpfsNode
func LegacyProviderOpt(reprovideInterval time.Duration, strategy string, acceleratedDHTClient bool, provideWorkerCount int) fx.Option {
	system := fx.Provide(
		fx.Annotate(func(lc fx.Lifecycle, cr irouting.ProvideManyRouter, repo repo.Repo) (*LegacyProvider, error) {
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
								KeysOnly: true,
							})
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
🔔🔔🔔 Reprovide Operations Too Slow 🔔🔔🔔

Your node may be falling behind on DHT reprovides, which could affect content availability.

Observed: %d keys at %v per key
Estimated: Assuming 10TiB blockstore, would take %v to complete
⏰ Must finish within %v (Provide.DHT.Interval)

Solutions (try in order):
1. Enable Provide.DHT.SweepEnabled=true (recommended)
2. Increase Provide.DHT.MaxWorkers if needed
3. Enable Routing.AcceleratedDHTClient=true (last resort, resource intensive)

Learn more: https://github.com/ipfs/kubo/blob/master/docs/config.md#provide`,
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
🔔🔔🔔 Reprovide Operations Too Slow 🔔🔔🔔

Your node is falling behind on DHT reprovides, which will affect content availability.

Observed: %d keys at %v per key
Confirmed: ~%d total CIDs requiring %v to complete
⏰ Must finish within %v (Provide.DHT.Interval)

Solutions (try in order):
1. Enable Provide.DHT.SweepEnabled=true (recommended)
2. Increase Provide.DHT.MaxWorkers if needed
3. Enable Routing.AcceleratedDHTClient=true (last resort, resource intensive)

Learn more: https://github.com/ipfs/kubo/blob/master/docs/config.md#provide`,
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

			prov := &LegacyProvider{sys}
			handleStrategyChange(strategy, prov, repo.Datastore())

			return prov, nil
		},
			fx.As(new(provider.System)),
			fx.As(new(DHTProvider)),
		),
	)
	setKeyProvider := fx.Invoke(func(lc fx.Lifecycle, system provider.System, keyProvider provider.KeyChanFunc) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				// SetKeyProvider breaks the circular dependency between provider, blockstore, and pinner.
				// We cannot create the blockstore without the provider (it needs to provide blocks),
				// and we cannot determine the reproviding strategy without the pinner/blockstore.
				// This deferred initialization allows us to create provider.System first,
				// then set the actual key provider function after all dependencies are ready.
				system.SetKeyProvider(keyProvider)
				return nil
			},
		})
	})
	return fx.Options(
		system,
		setKeyProvider,
	)
}

type dhtImpl interface {
	routing.Routing
	GetClosestPeers(context.Context, string) ([]peer.ID, error)
	Host() host.Host
	MessageSender() dht_pb.MessageSender
}
type addrsFilter interface {
	FilteredAddrs() []ma.Multiaddr
}

func SweepingProviderOpt(cfg *config.Config) fx.Option {
	reprovideInterval := cfg.Provide.DHT.Interval.WithDefault(config.DefaultProvideDHTInterval)
	type providerInput struct {
		fx.In
		DHT  routing.Routing `name:"dhtc"`
		Repo repo.Repo
	}
	sweepingReprovider := fx.Provide(func(in providerInput) (DHTProvider, *keystore.ResettableKeystore, error) {
		ds := namespace.Wrap(in.Repo.Datastore(), datastore.NewKey(providerDatastorePrefix))
		ks, err := keystore.NewResettableKeystore(ds,
			keystore.WithPrefixBits(16),
			keystore.WithDatastorePath(keystoreDatastorePath),
			keystore.WithBatchSize(int(cfg.Provide.DHT.KeystoreBatchSize.WithDefault(config.DefaultProvideDHTKeystoreBatchSize))),
		)
		if err != nil {
			return nil, nil, err
		}
		// Constants for buffered provider configuration
		// These values match the upstream defaults from go-libp2p-kad-dht and have been battle-tested
		const (
			// bufferedDsName is the datastore namespace used by the buffered provider.
			// The dsqueue persists operations here to handle large data additions without
			// being memory-bound, allowing operations on hardware with limited RAM and
			// enabling core operations to return instantly while processing happens async.
			bufferedDsName = "bprov"

			// bufferedBatchSize controls how many operations are dequeued and processed
			// together from the datastore queue. The worker processes up to this many
			// operations at once, grouping them by type for efficiency.
			bufferedBatchSize = 1 << 10 // 1024 items

			// bufferedIdleWriteTime is an implementation detail of go-dsqueue that controls
			// how long the datastore buffer waits for new multihashes to arrive before
			// flushing in-memory items to the datastore. This does NOT affect providing speed -
			// provides happen as fast as possible via a dedicated worker that continuously
			// processes the queue regardless of this timing.
			bufferedIdleWriteTime = time.Minute
		)

		bufferedProviderOpts := []buffered.Option{
			buffered.WithBatchSize(bufferedBatchSize),
			buffered.WithDsName(bufferedDsName),
			buffered.WithIdleWriteTime(bufferedIdleWriteTime),
		}
		var impl dhtImpl
		switch inDht := in.DHT.(type) {
		case *dht.IpfsDHT:
			if inDht != nil {
				impl = inDht
			}
		case *dual.DHT:
			if inDht != nil {
				prov, err := ddhtprovider.New(inDht,
					ddhtprovider.WithKeystore(ks),
					ddhtprovider.WithDatastore(ds),
					ddhtprovider.WithResumeCycle(cfg.Provide.DHT.ResumeEnabled.WithDefault(config.DefaultProvideDHTResumeEnabled)),

					ddhtprovider.WithReprovideInterval(reprovideInterval),
					ddhtprovider.WithMaxReprovideDelay(time.Hour),
					ddhtprovider.WithOfflineDelay(cfg.Provide.DHT.OfflineDelay.WithDefault(config.DefaultProvideDHTOfflineDelay)),
					ddhtprovider.WithConnectivityCheckOnlineInterval(1*time.Minute),

					ddhtprovider.WithMaxWorkers(int(cfg.Provide.DHT.MaxWorkers.WithDefault(config.DefaultProvideDHTMaxWorkers))),
					ddhtprovider.WithDedicatedPeriodicWorkers(int(cfg.Provide.DHT.DedicatedPeriodicWorkers.WithDefault(config.DefaultProvideDHTDedicatedPeriodicWorkers))),
					ddhtprovider.WithDedicatedBurstWorkers(int(cfg.Provide.DHT.DedicatedBurstWorkers.WithDefault(config.DefaultProvideDHTDedicatedBurstWorkers))),
					ddhtprovider.WithMaxProvideConnsPerWorker(int(cfg.Provide.DHT.MaxProvideConnsPerWorker.WithDefault(config.DefaultProvideDHTMaxProvideConnsPerWorker))),
				)
				if err != nil {
					return nil, nil, err
				}
				return buffered.New(prov, ds, bufferedProviderOpts...), ks, nil
			}
		case *fullrt.FullRT:
			if inDht != nil {
				impl = inDht
			}
		}
		if impl == nil {
			return &NoopProvider{}, nil, nil
		}

		var selfAddrsFunc func() []ma.Multiaddr
		if imlpFilter, ok := impl.(addrsFilter); ok {
			selfAddrsFunc = imlpFilter.FilteredAddrs
		} else {
			selfAddrsFunc = func() []ma.Multiaddr { return impl.Host().Addrs() }
		}
		opts := []dhtprovider.Option{
			dhtprovider.WithKeystore(ks),
			dhtprovider.WithDatastore(ds),
			dhtprovider.WithResumeCycle(cfg.Provide.DHT.ResumeEnabled.WithDefault(config.DefaultProvideDHTResumeEnabled)),
			dhtprovider.WithHost(impl.Host()),
			dhtprovider.WithRouter(impl),
			dhtprovider.WithMessageSender(impl.MessageSender()),
			dhtprovider.WithSelfAddrs(selfAddrsFunc),
			dhtprovider.WithAddLocalRecord(func(h mh.Multihash) error {
				return impl.Provide(context.Background(), cid.NewCidV1(cid.Raw, h), false)
			}),

			dhtprovider.WithReplicationFactor(amino.DefaultBucketSize),
			dhtprovider.WithReprovideInterval(reprovideInterval),
			dhtprovider.WithMaxReprovideDelay(time.Hour),
			dhtprovider.WithOfflineDelay(cfg.Provide.DHT.OfflineDelay.WithDefault(config.DefaultProvideDHTOfflineDelay)),
			dhtprovider.WithConnectivityCheckOnlineInterval(1 * time.Minute),

			dhtprovider.WithMaxWorkers(int(cfg.Provide.DHT.MaxWorkers.WithDefault(config.DefaultProvideDHTMaxWorkers))),
			dhtprovider.WithDedicatedPeriodicWorkers(int(cfg.Provide.DHT.DedicatedPeriodicWorkers.WithDefault(config.DefaultProvideDHTDedicatedPeriodicWorkers))),
			dhtprovider.WithDedicatedBurstWorkers(int(cfg.Provide.DHT.DedicatedBurstWorkers.WithDefault(config.DefaultProvideDHTDedicatedBurstWorkers))),
			dhtprovider.WithMaxProvideConnsPerWorker(int(cfg.Provide.DHT.MaxProvideConnsPerWorker.WithDefault(config.DefaultProvideDHTMaxProvideConnsPerWorker))),
		}

		prov, err := dhtprovider.New(opts...)
		if err != nil {
			return nil, nil, err
		}
		return buffered.New(prov, ds, bufferedProviderOpts...), ks, nil
	})

	type keystoreInput struct {
		fx.In
		Provider    DHTProvider
		Keystore    *keystore.ResettableKeystore
		KeyProvider provider.KeyChanFunc
	}
	initKeystore := fx.Invoke(func(lc fx.Lifecycle, in keystoreInput) {
		// Skip keystore initialization for NoopProvider
		if _, ok := in.Provider.(*NoopProvider); ok {
			return
		}

		var (
			cancel context.CancelFunc
			done   = make(chan struct{})
		)

		syncKeystore := func(ctx context.Context) error {
			kcf, err := in.KeyProvider(ctx)
			if err != nil {
				return err
			}
			if err := in.Keystore.ResetCids(ctx, kcf); err != nil {
				return err
			}
			if err := in.Provider.RefreshSchedule(); err != nil {
				logger.Infow("refreshing provider schedule", "err", err)
			}
			return nil
		}

		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				// Set the KeyProvider as a garbage collection function for the
				// keystore. Periodically purge the Keystore from all its keys and
				// replace them with the keys that needs to be reprovided, coming from
				// the KeyChanFunc. So far, this is the less worse way to remove CIDs
				// that shouldn't be reprovided from the provider's state.
				go func() {
					// Sync the keystore once at startup. This operation is async since
					// we need to walk the DAG of objects matching the provide strategy,
					// which can take a while.
					strategy := cfg.Provide.Strategy.WithDefault(config.DefaultProvideStrategy)
					logger.Infow("provider keystore sync started", "strategy", strategy)
					if err := syncKeystore(ctx); err != nil {
						logger.Errorw("provider keystore sync failed", "err", err, "strategy", strategy)
					} else {
						logger.Infow("provider keystore sync completed", "strategy", strategy)
					}
				}()

				gcCtx, c := context.WithCancel(context.Background())
				cancel = c

				go func() { // garbage collection loop for cids to reprovide
					defer close(done)
					ticker := time.NewTicker(reprovideInterval)
					defer ticker.Stop()

					for {
						select {
						case <-gcCtx.Done():
							return
						case <-ticker.C:
							if err := syncKeystore(gcCtx); err != nil {
								logger.Errorw("provider keystore sync", "err", err)
							}
						}
					}
				}()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if cancel != nil {
					cancel()
				}
				select {
				case <-done:
				case <-ctx.Done():
					return ctx.Err()
				}
				// Keystore data isn't purged, on close, but it will be overwritten
				// when the node starts again.
				return in.Keystore.Close()
			},
		})
	})

	// extractSweepingProvider extracts a SweepingProvider from the given provider interface.
	// It handles unwrapping buffered and dual providers, always selecting WAN for dual DHT.
	// Returns nil if the provider is not a sweeping provider type.
	var extractSweepingProvider func(prov any) *dhtprovider.SweepingProvider
	extractSweepingProvider = func(prov any) *dhtprovider.SweepingProvider {
		switch p := prov.(type) {
		case *dhtprovider.SweepingProvider:
			return p
		case *ddhtprovider.SweepingProvider:
			return p.WAN
		case *buffered.SweepingProvider:
			// Recursively extract from the inner provider
			return extractSweepingProvider(p.Provider)
		default:
			return nil
		}
	}

	type alertInput struct {
		fx.In
		Provider DHTProvider
	}
	reprovideAlert := fx.Invoke(func(lc fx.Lifecycle, in alertInput) {
		prov := extractSweepingProvider(in.Provider)

		var (
			cancel context.CancelFunc
			done   = make(chan struct{})
		)

		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				if prov == nil {
					return nil
				}
				gcCtx, c := context.WithCancel(context.Background())
				cancel = c
				go func() {
					defer close(done)

					ticker := time.NewTicker(reprovideAlertPollInterval)
					defer ticker.Stop()

					var (
						queueSize, prevQueueSize         int64
						queuedWorkers, prevQueuedWorkers bool
						count                            int
					)

					for {
						select {
						case <-gcCtx.Done():
							return
						case <-ticker.C:
						}

						stats := prov.Stats()
						queuedWorkers = stats.Workers.QueuedPeriodic > 0
						queueSize = int64(stats.Queues.PendingRegionReprovides)

						// Alert if reprovide queue keeps growing and all periodic workers are busy.
						// Requires consecutiveAlertsThreshold intervals of sustained growth.
						if prevQueuedWorkers && queuedWorkers && queueSize > prevQueueSize {
							count++
							if count >= consecutiveAlertsThreshold {
								logger.Errorf(`
🔔🔔🔔 Reprovide Operations Too Slow 🔔🔔🔔

Your node is falling behind on DHT reprovides, which will affect content availability.

Keyspace regions enqueued for reprovide:
  %s ago:\t%d
  Now:\t%d

All periodic workers are busy!
  Active workers:\t%d / %d (max)
  Active workers types:\t%d periodic, %d burst
  Dedicated workers:\t%d periodic, %d burst

Solutions (try in order):
1. Increase Provide.DHT.MaxWorkers (current %d)
2. Increase Provide.DHT.DedicatedPeriodicWorkers (current %d)
3. Set Provide.DHT.SweepEnabled=false and Routing.AcceleratedDHTClient=true (last resort, not recommended)

See how the reprovide queue is processed in real-time with 'watch ipfs provide stat --all --compact'

See docs: https://github.com/ipfs/kubo/blob/master/docs/config.md#providedhtmaxworkers`,
									reprovideAlertPollInterval.Truncate(time.Minute).String(), prevQueueSize, queueSize,
									stats.Workers.Active, stats.Workers.Max,
									stats.Workers.ActivePeriodic, stats.Workers.ActiveBurst,
									stats.Workers.DedicatedPeriodic, stats.Workers.DedicatedBurst,
									stats.Workers.Max, stats.Workers.DedicatedPeriodic)
							}
						} else if !queuedWorkers {
							count = 0
						}

						prevQueueSize, prevQueuedWorkers = queueSize, queuedWorkers
					}
				}()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				// Cancel the alert loop
				if cancel != nil {
					cancel()
				}
				select {
				case <-done:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			},
		})
	})

	return fx.Options(
		sweepingReprovider,
		initKeystore,
		reprovideAlert,
	)
}

// ONLINE/OFFLINE

// OnlineProviders groups units managing provide routing records online
func OnlineProviders(provide bool, cfg *config.Config) fx.Option {
	if !provide {
		return OfflineProviders()
	}

	providerStrategy := cfg.Provide.Strategy.WithDefault(config.DefaultProvideStrategy)

	strategyFlag := config.ParseProvideStrategy(providerStrategy)
	if strategyFlag == 0 {
		return fx.Error(fmt.Errorf("provider: unknown strategy %q", providerStrategy))
	}

	opts := []fx.Option{
		fx.Provide(setReproviderKeyProvider(providerStrategy)),
	}
	if cfg.Provide.DHT.SweepEnabled.WithDefault(config.DefaultProvideDHTSweepEnabled) {
		opts = append(opts, SweepingProviderOpt(cfg))
	} else {
		reprovideInterval := cfg.Provide.DHT.Interval.WithDefault(config.DefaultProvideDHTInterval)
		acceleratedDHTClient := cfg.Routing.AcceleratedDHTClient.WithDefault(config.DefaultAcceleratedDHTClient)
		provideWorkerCount := int(cfg.Provide.DHT.MaxWorkers.WithDefault(config.DefaultProvideDHTMaxWorkers))

		opts = append(opts, LegacyProviderOpt(reprovideInterval, providerStrategy, acceleratedDHTClient, provideWorkerCount))
	}

	return fx.Options(opts...)
}

// OfflineProviders groups units managing provide routing records offline
func OfflineProviders() fx.Option {
	return fx.Provide(func() DHTProvider {
		return &NoopProvider{}
	})
}

func mfsProvider(mfsRoot *mfs.Root, fetcher fetcher.Factory) provider.KeyChanFunc {
	return func(ctx context.Context) (<-chan cid.Cid, error) {
		err := mfsRoot.FlushMemFree(ctx)
		if err != nil {
			return nil, fmt.Errorf("provider: error flushing MFS, cannot provide MFS: %w", err)
		}
		rootNode, err := mfsRoot.GetDirectory().GetNode()
		if err != nil {
			return nil, fmt.Errorf("provider: error loading MFS root, cannot provide MFS: %w", err)
		}

		kcf := provider.NewDAGProvider(rootNode.Cid(), fetcher)
		return kcf(ctx)
	}
}

type provStrategyIn struct {
	fx.In
	Pinner               pin.Pinner
	Blockstore           blockstore.Blockstore
	OfflineIPLDFetcher   fetcher.Factory `name:"offlineIpldFetcher"`
	OfflineUnixFSFetcher fetcher.Factory `name:"offlineUnixfsFetcher"`
	MFSRoot              *mfs.Root
	Repo                 repo.Repo
}

type provStrategyOut struct {
	fx.Out
	ProvidingStrategy    config.ProvideStrategy
	ProvidingKeyChanFunc provider.KeyChanFunc
}

// createKeyProvider creates the appropriate KeyChanFunc based on strategy.
// Each strategy has different behavior:
// - "roots": Only root CIDs of pinned content
// - "pinned": All pinned content (roots + children)
// - "mfs": Only MFS content
// - "all": all blocks
func createKeyProvider(strategyFlag config.ProvideStrategy, in provStrategyIn) provider.KeyChanFunc {
	switch strategyFlag {
	case config.ProvideStrategyRoots:
		return provider.NewBufferedProvider(dspinner.NewPinnedProvider(true, in.Pinner, in.OfflineIPLDFetcher))
	case config.ProvideStrategyPinned:
		return provider.NewBufferedProvider(dspinner.NewPinnedProvider(false, in.Pinner, in.OfflineIPLDFetcher))
	case config.ProvideStrategyPinned | config.ProvideStrategyMFS:
		return provider.NewPrioritizedProvider(
			provider.NewBufferedProvider(dspinner.NewPinnedProvider(false, in.Pinner, in.OfflineIPLDFetcher)),
			mfsProvider(in.MFSRoot, in.OfflineUnixFSFetcher),
		)
	case config.ProvideStrategyMFS:
		return mfsProvider(in.MFSRoot, in.OfflineUnixFSFetcher)
	default: // "all", "", "flat" (compat)
		return in.Blockstore.AllKeysChan
	}
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
func handleStrategyChange(strategy string, provider DHTProvider, ds datastore.Datastore) {
	ctx := context.Background()

	previous, changed, err := detectStrategyChange(ctx, strategy, ds)
	if err != nil {
		logger.Error("cannot read previous reprovide strategy", "err", err)
		return
	}

	if !changed {
		return
	}

	logger.Infow("Provide.Strategy changed, clearing provide queue", "previous", previous, "current", strategy)
	provider.Clear()

	if err := persistStrategy(ctx, strategy, ds); err != nil {
		logger.Error("cannot update reprovide strategy", "err", err)
	}
}

func setReproviderKeyProvider(strategy string) func(in provStrategyIn) provStrategyOut {
	strategyFlag := config.ParseProvideStrategy(strategy)

	return func(in provStrategyIn) provStrategyOut {
		// Create the appropriate key provider based on strategy
		kcf := createKeyProvider(strategyFlag, in)
		return provStrategyOut{
			ProvidingStrategy:    strategyFlag,
			ProvidingKeyChanFunc: kcf,
		}
	}
}
