package node

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/dag/walker"
	"github.com/ipfs/boxo/fetcher"
	"github.com/ipfs/boxo/mfs"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/boxo/pinning/pinner/dspinner"
	"github.com/ipfs/boxo/provider"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/mount"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
	log "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/shutdown"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
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

	// KeystoreDatastorePath is the base directory for the provider keystore datastores.
	KeystoreDatastorePath = "provider-keystore"

	// reprovideLastUniqueCountKey stores the unique CID count from
	// the last +unique reprovide cycle, used to size the next cycle's
	// bloom filter.
	reprovideLastUniqueCountKey = "/reprovideLastUniqueCount"
)

var (
	// Datastore namespace key for provider data.
	providerDatastoreKey = datastore.NewKey("provider")
	// Datastore namespace key for provider keystore data.
	keystoreDatastoreKey = datastore.NewKey("keystore")
)

// providerLog is the go-log subsystem used for provide/reprovide-related
// messages emitted from kubo's own orchestration code. It shares the
// "provider" subsystem name with boxo's provider package so users can set
// GOLOG_LOG_LEVEL=provider=<level> to control both layers at once. See
// docs/debug-guide.md for the full list of provide-related subsystems.
var providerLog = log.Logger("provider")

var errAcceleratedDHTNotReady = errors.New("AcceleratedDHTClient: routing table not ready")

// validateKeystoreSuffix rejects any suffix other than "0" or "1".
// The upstream library uses these two values as alternating namespace
// identifiers. Validating here prevents accidental deletion of unrelated
// directories via os.RemoveAll if the upstream ever changes its scheme.
func validateKeystoreSuffix(suffix string) error {
	if suffix != "0" && suffix != "1" {
		return fmt.Errorf("unexpected keystore suffix %q, expected \"0\" or \"1\"", suffix)
	}
	return nil
}

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
	Close() error
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
func (r *NoopProvider) Close() error                               { return nil }

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
								providerLog.Errorf("fetching AllKeysChain in provider ThroughputReport: %v", err)
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
										providerLog.Errorf(`
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
							providerLog.Errorf(`
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
					return shutdown.CloseWithCtx(ctx, "legacy-provider", sys.Close)
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

type fullrtRouter struct {
	*fullrt.FullRT
	ready  bool
	logger *log.ZapEventLogger
}

func newFullRTRouter(fr *fullrt.FullRT, loggerName string) *fullrtRouter {
	return &fullrtRouter{
		FullRT: fr,
		ready:  true,
		logger: log.Logger(loggerName),
	}
}

// GetClosestPeers overrides fullrt.FullRT's GetClosestPeers and returns an
// error if the fullrt's initial network crawl isn't complete yet.
func (fr *fullrtRouter) GetClosestPeers(ctx context.Context, key string) ([]peer.ID, error) {
	if fr.ready {
		if !fr.Ready() {
			fr.ready = false
			fr.logger.Info("AcceleratedDHTClient: waiting for routing table initialization (5-10 min, depends on DHT size and network) to complete before providing")
			return nil, errAcceleratedDHTNotReady
		}
	} else {
		if fr.Ready() {
			fr.ready = true
			fr.logger.Info("AcceleratedDHTClient: routing table ready, providing can begin")
		} else {
			return nil, errAcceleratedDHTNotReady
		}
	}
	return fr.FullRT.GetClosestPeers(ctx, key)
}

var (
	_ dhtImpl = &dht.IpfsDHT{}
	_ dhtImpl = &fullrtRouter{}
)

type addrsFilter interface {
	FilteredAddrs() []ma.Multiaddr
}

// findRootDatastoreSpec extracts the leaf datastore spec for the root ("/")
// mount from the repo's Datastore.Spec config. It unwraps mount (picks the "/"
// mountpoint), measure, and log wrappers to find the actual backend spec
// (e.g., levelds, pebbleds).
func findRootDatastoreSpec(spec map[string]any) map[string]any {
	if spec == nil {
		return nil
	}
	switch spec["type"] {
	case "mount":
		mounts, ok := spec["mounts"].([]any)
		if !ok {
			return spec
		}
		for _, m := range mounts {
			mnt, ok := m.(map[string]any)
			if !ok {
				continue
			}
			if mnt["mountpoint"] == "/" {
				return findRootDatastoreSpec(mnt)
			}
		}
		// No root mount found; return nil so callers fall back gracefully
		// (in-memory datastore or skip mounting) rather than passing a
		// mount-type spec to openDatastoreAt which expects a leaf backend.
		return nil
	case "measure", "log":
		if child, ok := spec["child"].(map[string]any); ok {
			return findRootDatastoreSpec(child)
		}
		return spec
	default:
		if _, hasChild := spec["child"]; hasChild {
			providerLog.Warnw("unrecognized datastore wrapper type, using as-is",
				"type", spec["type"])
		}
		return spec
	}
}

// MountKeystoreDatastores opens any provider keystore datastores that exist on
// disk and returns them as mount.Mount entries ready to be combined with the
// main repo datastore. The caller must call the returned cleanup function when
// done. Returns nil mounts and a no-op closer if no keystores exist.
func MountKeystoreDatastores(repo repo.Repo) ([]mount.Mount, func(), error) {
	cfg, err := repo.Config()
	if err != nil {
		return nil, nil, fmt.Errorf("reading repo config: %w", err)
	}

	rootSpec := findRootDatastoreSpec(cfg.Datastore.Spec)
	if rootSpec == nil {
		return nil, func() {}, nil
	}

	keystoreBasePath := filepath.Join(repo.Path(), KeystoreDatastorePath)
	var mounts []mount.Mount
	var closers []func()

	for _, suffix := range []string{"0", "1"} {
		dir := filepath.Join(keystoreBasePath, suffix)
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		ds, err := openDatastoreAt(rootSpec, dir)
		if err != nil {
			for _, c := range closers {
				c()
			}
			return nil, nil, err
		}
		prefix := providerDatastoreKey.Child(keystoreDatastoreKey).ChildString(suffix)
		mounts = append(mounts, mount.Mount{Prefix: prefix, Datastore: ds})
		closers = append(closers, func() { ds.Close() })
	}

	closer := func() {
		for _, c := range closers {
			c()
		}
	}
	return mounts, closer, nil
}

// openDatastoreAt opens a datastore using the given spec at the specified path.
// It deep-copies the spec to avoid mutating the original.
func openDatastoreAt(rootSpec map[string]any, path string) (datastore.Batching, error) {
	spec := copySpec(rootSpec)
	spec["path"] = path
	dsc, err := fsrepo.AnyDatastoreConfig(spec)
	if err != nil {
		return nil, fmt.Errorf("creating datastore config for %s: %w", path, err)
	}
	return dsc.Create("")
}

// copySpec deep-copies a datastore spec map so modifications (e.g., changing
// the path) don't affect the original.
func copySpec(spec map[string]any) map[string]any {
	if spec == nil {
		return nil
	}
	cp := make(map[string]any, len(spec))
	for k, v := range spec {
		switch val := v.(type) {
		case map[string]any:
			cp[k] = copySpec(val)
		case []any:
			s := make([]any, len(val))
			for i, elem := range val {
				if m, ok := elem.(map[string]any); ok {
					s[i] = copySpec(m)
				} else {
					s[i] = elem
				}
			}
			cp[k] = s
		default:
			cp[k] = v
		}
	}
	return cp
}

// purgeBatchSize is the number of keys deleted per batch commit during
// orphaned keystore cleanup. Each commit is a cancellation checkpoint.
const purgeBatchSize = 1 << 12 // 4096

// purgeOrphanedKeystoreData deletes all keys under /provider/keystore/ from the
// shared repo datastore. These were written by older Kubo versions that stored
// provider keystore data inline in the shared datastore. The new code uses
// separate filesystem datastores under <repo>/{KeystoreDatastorePath}/ instead.
//
// The operation is idempotent and safe to interrupt: partial completion is
// fine because already-deleted keys are no-ops on re-run.
func purgeOrphanedKeystoreData(ctx context.Context, ds datastore.Batching) error {
	orphanedPrefix := providerDatastoreKey.Child(keystoreDatastoreKey).String()
	syncKey := datastore.NewKey(orphanedPrefix)

	results, err := ds.Query(ctx, query.Query{
		Prefix:   orphanedPrefix,
		KeysOnly: true,
	})
	if err != nil {
		return fmt.Errorf("querying orphaned keystore data: %w", err)
	}
	defer results.Close()

	var batch datastore.Batch
	var count, pending int
	for result := range results.Next() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if result.Error != nil {
			return fmt.Errorf("iterating orphaned keystore data: %w", result.Error)
		}
		if batch == nil {
			batch, err = ds.Batch(ctx)
			if err != nil {
				return fmt.Errorf("creating batch for orphaned keystore cleanup: %w", err)
			}
		}
		if err := batch.Delete(ctx, datastore.NewKey(result.Key)); err != nil {
			return fmt.Errorf("batch deleting orphaned key %s: %w", result.Key, err)
		}
		count++
		pending++
		if pending >= purgeBatchSize {
			if err := batch.Commit(ctx); err != nil {
				return fmt.Errorf("committing orphaned keystore cleanup batch: %w", err)
			}
			if err := ds.Sync(ctx, syncKey); err != nil {
				return fmt.Errorf("syncing orphaned keystore cleanup: %w", err)
			}
			batch = nil
			pending = 0
		}
	}
	if pending > 0 {
		if err := batch.Commit(ctx); err != nil {
			return fmt.Errorf("committing orphaned keystore cleanup batch: %w", err)
		}
		if err := ds.Sync(ctx, syncKey); err != nil {
			return fmt.Errorf("syncing orphaned keystore cleanup: %w", err)
		}
	}
	if count > 0 {
		providerLog.Infow("purged orphaned provider keystore data from shared datastore", "keys", count)
	}
	return nil
}

func SweepingProviderOpt(cfg *config.Config) fx.Option {
	reprovideInterval := cfg.Provide.DHT.Interval.WithDefault(config.DefaultProvideDHTInterval)
	// noScheduleMode is true when the user disabled the periodic reprovide
	// schedule (Provide.DHT.Interval=0). In this mode the keystore is
	// inert: kad-dht's burst-only path (ProvideOnce, StartProviding) does
	// not Put or Delete keys, and no reprovide loop runs to read them.
	noScheduleMode := reprovideInterval == 0
	type providerInput struct {
		fx.In
		DHT  routing.Routing `name:"dhtc"`
		Repo repo.Repo
		Lc   fx.Lifecycle
	}
	sweepingReprovider := fx.Provide(func(in providerInput) (DHTProvider, *keystore.ResettableKeystore, error) {
		ds := namespace.Wrap(in.Repo.Datastore(), providerDatastoreKey)

		// Get repo path and config to determine datastore type
		repoPath := in.Repo.Path()
		repoCfg, err := in.Repo.Config()
		if err != nil {
			return nil, nil, fmt.Errorf("getting repo config: %w", err)
		}

		// Find the root datastore type (levelds, pebbleds, etc.)
		rootSpec := findRootDatastoreSpec(repoCfg.Datastore.Spec)

		// Keystore datastores live at <repo>/{KeystoreDatastorePath}/<suffix>
		keystoreBasePath := filepath.Join(repoPath, KeystoreDatastorePath)

		createDs := func(suffix string) (datastore.Batching, error) {
			if err := validateKeystoreSuffix(suffix); err != nil {
				return nil, err
			}
			// In-memory datastore in no-schedule mode (keystore is inert)
			// or when no datastore spec is configured (test/mock repos).
			if noScheduleMode || rootSpec == nil {
				return datastore.NewMapDatastore(), nil
			}
			if err := os.MkdirAll(keystoreBasePath, 0o755); err != nil {
				return nil, fmt.Errorf("creating keystore base directory: %w", err)
			}
			ds, err := openDatastoreAt(rootSpec, filepath.Join(keystoreBasePath, suffix))
			if err != nil {
				return nil, err
			}
			providerLog.Infow("provider keystore: opened datastore", "suffix", suffix, "path", filepath.Join(keystoreBasePath, suffix))
			return ds, nil
		}

		destroyDs := func(suffix string) error {
			if err := validateKeystoreSuffix(suffix); err != nil {
				return err
			}
			if noScheduleMode {
				return nil
			}
			providerLog.Infow("provider keystore: removing datastore from disk", "suffix", suffix, "path", filepath.Join(keystoreBasePath, suffix))
			return os.RemoveAll(filepath.Join(keystoreBasePath, suffix))
		}

		// In no-schedule mode the on-disk keystore is never used. If a
		// previous run was in schedule mode it may have left data behind;
		// purge it once on startup to free disk.
		if noScheduleMode {
			if _, statErr := os.Stat(keystoreBasePath); statErr == nil {
				providerLog.Infow("provider keystore: purging on-disk data (Provide.DHT.Interval=0)", "path", keystoreBasePath)
				if rmErr := os.RemoveAll(keystoreBasePath); rmErr != nil {
					providerLog.Warnw("provider keystore: purge failed", "path", keystoreBasePath, "err", rmErr)
				}
			}
		}

		// One-time cleanup of stale keystore data left by older Kubo in the
		// shared repo datastore under /provider/keystore/. New code stores
		// bulk key data in separate filesystem datastores under
		// <repo>/{KeystoreDatastorePath}/ while still using the same
		// /provider/keystore/ namespace in the shared datastore for metadata.
		//
		// The absence of the keystoreBasePath directory signals a first run
		// after upgrade: the directory is created later by createDs on first
		// use, so it doubles as a "cleanup done" flag. If the process dies
		// mid-purge the directory still won't exist and the cleanup re-runs
		// on next start (it is idempotent). Must run synchronously before
		// NewResettableKeystore to avoid racing with reads on the same
		// namespace.
		if _, statErr := os.Stat(keystoreBasePath); os.IsNotExist(statErr) {
			providerLog.Infow("migrating provider keystore data from shared datastore to separate filesystem datastores", "path", keystoreBasePath)
			// Create a cancellable context for the purge. The OnStop hook
			// below calls purgeCancel when the node receives a shutdown
			// signal (e.g., SIGINT), which interrupts the purge loop
			// instead of blocking indefinitely.
			purgeCtx, purgeCancel := context.WithCancel(context.Background())
			in.Lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					purgeCancel()
					return nil
				},
			})
			if purgeErr := purgeOrphanedKeystoreData(purgeCtx, in.Repo.Datastore()); purgeErr != nil {
				if purgeCtx.Err() != nil {
					providerLog.Infow("provider keystore migration interrupted by shutdown, will resume on next start")
				} else {
					providerLog.Warnw("provider keystore migration failed, will retry on next start", "error", purgeErr)
				}
			} else {
				providerLog.Infow("provider keystore migration completed")
			}
			purgeCancel()
		}

		keystoreDs := namespace.Wrap(ds, keystoreDatastoreKey)
		ks, err := keystore.NewResettableKeystore(keystoreDs,
			keystore.WithDatastoreFactory(createDs, destroyDs),
			keystore.KeystoreOption(
				keystore.WithPrefixBits(16),
				keystore.WithBatchSize(int(cfg.Provide.DHT.KeystoreBatchSize.WithDefault(config.DefaultProvideDHTKeystoreBatchSize))),
			),
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

			// loggerName is the name of the go-log logger used by the provider.
			loggerName = dhtprovider.DefaultLoggerName
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
					ddhtprovider.WithSendProviderRecordTimeout(cfg.Provide.DHT.SendProviderRecordTimeout.WithDefault(config.DefaultProvideDHTSendProviderRecordTimeout)),

					ddhtprovider.WithMaxWorkers(int(cfg.Provide.DHT.MaxWorkers.WithDefault(config.DefaultProvideDHTMaxWorkers))),
					ddhtprovider.WithDedicatedPeriodicWorkers(int(cfg.Provide.DHT.DedicatedPeriodicWorkers.WithDefault(config.DefaultProvideDHTDedicatedPeriodicWorkers))),
					ddhtprovider.WithDedicatedBurstWorkers(int(cfg.Provide.DHT.DedicatedBurstWorkers.WithDefault(config.DefaultProvideDHTDedicatedBurstWorkers))),
					ddhtprovider.WithMaxProvideConnsPerWorker(int(cfg.Provide.DHT.MaxProvideConnsPerWorker.WithDefault(config.DefaultProvideDHTMaxProvideConnsPerWorker))),

					ddhtprovider.WithLoggerName(loggerName),
				)
				if err != nil {
					return nil, nil, err
				}
				return buffered.New(prov, ds, bufferedProviderOpts...), ks, nil
			}
		case *fullrt.FullRT:
			if inDht != nil {
				impl = newFullRTRouter(inDht, loggerName)
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
			dhtprovider.WithSendProviderRecordTimeout(cfg.Provide.DHT.SendProviderRecordTimeout.WithDefault(config.DefaultProvideDHTSendProviderRecordTimeout)),

			dhtprovider.WithMaxWorkers(int(cfg.Provide.DHT.MaxWorkers.WithDefault(config.DefaultProvideDHTMaxWorkers))),
			dhtprovider.WithDedicatedPeriodicWorkers(int(cfg.Provide.DHT.DedicatedPeriodicWorkers.WithDefault(config.DefaultProvideDHTDedicatedPeriodicWorkers))),
			dhtprovider.WithDedicatedBurstWorkers(int(cfg.Provide.DHT.DedicatedBurstWorkers.WithDefault(config.DefaultProvideDHTDedicatedBurstWorkers))),
			dhtprovider.WithMaxProvideConnsPerWorker(int(cfg.Provide.DHT.MaxProvideConnsPerWorker.WithDefault(config.DefaultProvideDHTMaxProvideConnsPerWorker))),

			dhtprovider.WithLoggerName(loggerName),
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
		// In no-schedule mode no reprovide loop runs, so there is no
		// reader for the keystore and no need to sync it. The zero
		// interval would also panic the periodic sync ticker.
		if noScheduleMode {
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
				providerLog.Infow("refreshing provider schedule", "err", err)
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
					providerLog.Infow("provider keystore sync started", "strategy", strategy)
					if err := syncKeystore(ctx); err != nil {
						// ErrClosed means the keystore was closed by the shutdown
						// hook while this goroutine was still in flight: the
						// OnStart ctx is not cancelled yet, so we classify the
						// failure as shutdown explicitly.
						if ctx.Err() != nil || errors.Is(err, keystore.ErrClosed) {
							providerLog.Debugw("provider keystore sync interrupted by shutdown", "err", err, "strategy", strategy)
						} else {
							providerLog.Errorw("provider keystore sync failed", "err", err, "strategy", strategy)
						}
						return
					}
					providerLog.Infow("provider keystore sync completed", "strategy", strategy)
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
								if gcCtx.Err() != nil || errors.Is(err, keystore.ErrClosed) {
									providerLog.Debugw("provider keystore sync interrupted by shutdown", "err", err)
								} else {
									providerLog.Errorw("provider keystore sync failed", "err", err)
								}
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
				// Keystore will be closed by ensureProviderClosesBeforeKeystore hook
				// to guarantee provider closes before keystore.
				return nil
			},
		})
	})

	// ensureProviderClosesBeforeKeystore manages the shutdown order between
	// provider and keystore to prevent race conditions.
	//
	// The provider's worker goroutines may call keystore methods during their
	// operation. If keystore closes while these operations are in-flight, we get
	// "keystore is closed" errors. By closing the provider first, we ensure all
	// worker goroutines exit and complete any pending keystore operations before
	// the keystore itself closes.
	type providerKeystoreShutdownInput struct {
		fx.In
		Provider DHTProvider
		Keystore *keystore.ResettableKeystore
	}
	ensureProviderClosesBeforeKeystore := fx.Invoke(func(lc fx.Lifecycle, in providerKeystoreShutdownInput) {
		// Skip for NoopProvider
		if _, ok := in.Provider.(*NoopProvider); ok {
			return
		}

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				// Close provider first; waits for all worker goroutines
				// to exit so nothing can access the keystore after this
				// returns. If ctx fires before provider drains, the
				// keystore close below sees an expired ctx and returns
				// immediately; the watchdog is the ultimate backstop.
				if err := shutdown.CloseWithCtx(ctx, "dht-provider", in.Provider.Close); err != nil {
					providerLog.Errorw("error closing provider during shutdown", "error", err)
				}
				return shutdown.CloseWithCtx(ctx, "keystore", in.Keystore.Close)
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
		if prov == nil {
			return
		}

		var (
			cancel context.CancelFunc
			done   = make(chan struct{})
		)

		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
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

						statsCtx, statsCancel := context.WithTimeout(gcCtx, time.Minute)
						stats, err := prov.Stats(statsCtx)
						statsCancel()
						if err != nil {
							if gcCtx.Err() != nil {
								return
							}
							providerLog.Debugw("provider stats unavailable for reprovide alert", "err", err)
							continue
						}
						queuedWorkers = stats.Workers.QueuedPeriodic > 0
						queueSize = int64(stats.Queues.PendingRegionReprovides)

						// Alert if reprovide queue keeps growing and all periodic workers are busy.
						// Requires consecutiveAlertsThreshold intervals of sustained growth.
						if prevQueuedWorkers && queuedWorkers && queueSize > prevQueueSize {
							count++
							if count >= consecutiveAlertsThreshold {
								providerLog.Errorf(`
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
		ensureProviderClosesBeforeKeystore,
		reprovideAlert,
	)
}

// ONLINE/OFFLINE

// hasDHTRouting checks if the routing configuration includes a DHT component.
// Returns false for HTTP-only custom routing configurations (e.g., Routing.Type="custom"
// with only HTTP routers). This is used to determine whether SweepingProviderOpt
// can be used, since it requires a DHT client.
func hasDHTRouting(cfg *config.Config) bool {
	routingType := cfg.Routing.Type.WithDefault(config.DefaultRoutingType)
	switch routingType {
	case "auto", "autoclient", "dht", "dhtclient", "dhtserver":
		return true
	case "custom":
		// Check if any router in custom config is DHT-based
		for _, router := range cfg.Routing.Routers {
			if routerIncludesDHT(router, cfg) {
				return true
			}
		}
		return false
	default: // "none", "delegated"
		return false
	}
}

// routerIncludesDHT recursively checks if a router configuration includes DHT.
// Handles parallel and sequential composite routers by checking their children.
func routerIncludesDHT(rp config.RouterParser, cfg *config.Config) bool {
	switch rp.Type {
	case config.RouterTypeDHT:
		return true
	case config.RouterTypeParallel, config.RouterTypeSequential:
		if children, ok := rp.Parameters.(*config.ComposableRouterParams); ok {
			for _, child := range children.Routers {
				if childRouter, exists := cfg.Routing.Routers[child.RouterName]; exists {
					if routerIncludesDHT(childRouter, cfg) {
						return true
					}
				}
			}
		}
	}
	return false
}

// OnlineProviders groups units managing provide routing records online
func OnlineProviders(provide bool, cfg *config.Config) fx.Option {
	if !provide {
		return OfflineProviders()
	}

	providerStrategy := cfg.Provide.Strategy.WithDefault(config.DefaultProvideStrategy)

	if _, err := config.ParseProvideStrategy(providerStrategy); err != nil {
		return fx.Error(fmt.Errorf("provider: %w", err))
	}

	bloomFPRate := uint(cfg.Provide.BloomFPRate.WithDefault(config.DefaultProvideBloomFPRate))

	opts := []fx.Option{
		fx.Provide(setReproviderKeyProvider(providerStrategy, bloomFPRate)),
	}

	sweepEnabled := cfg.Provide.DHT.SweepEnabled.WithDefault(config.DefaultProvideDHTSweepEnabled)
	dhtAvailable := hasDHTRouting(cfg)

	// Use SweepingProvider only when both sweep is enabled AND DHT is available.
	// For HTTP-only routing (e.g., Routing.Type="custom" with only HTTP routers),
	// fall back to LegacyProvider which works with ProvideManyRouter.
	// See https://github.com/ipfs/kubo/issues/11089
	if sweepEnabled && dhtAvailable {
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

// readLastUniqueCount reads the persisted unique CID count from the
// previous +unique reprovide cycle. Returns 0 if not found or corrupt.
func readLastUniqueCount(ds datastore.Datastore) uint64 {
	val, err := ds.Get(context.Background(), datastore.NewKey(reprovideLastUniqueCountKey))
	if err != nil {
		return 0
	}
	if len(val) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(val)
}

// persistUniqueCount stores the unique CID count for the next cycle.
func persistUniqueCount(ds datastore.Datastore, count uint64) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, count)
	if err := ds.Put(context.Background(), datastore.NewKey(reprovideLastUniqueCountKey), buf); err != nil {
		providerLog.Errorf("failed to persist unique count: %s", err)
	}
}

// walkFunc abstracts a DAG walk (WalkDAG or WalkEntityRoots) so the
// MFS provider can be parameterized without duplicating the
// flush+walk+channel boilerplate.
type walkFunc func(ctx context.Context, root cid.Cid, emit func(cid.Cid) bool, opts ...walker.Option) error

// uniqueMFSProvider is the +unique counterpart of mfsProvider. It
// flushes the MFS root, then walks the MFS DAG with a shared
// VisitedTracker and a locality check (blockstore.Has) so only
// locally-present blocks are emitted.
func uniqueMFSProvider(mfsRoot *mfs.Root, bs blockstore.Blockstore, tracker walker.VisitedTracker) provider.KeyChanFunc {
	walk := func(ctx context.Context, root cid.Cid, emit func(cid.Cid) bool, opts ...walker.Option) error {
		return walker.WalkDAG(ctx, root, walker.LinksFetcherFromBlockstore(bs), emit, opts...)
	}
	return mfsWalkProvider(mfsRoot, bs, tracker, walk)
}

// mfsEntityRootsProvider is the +entities counterpart. It walks with
// WalkEntityRoots, emitting only entity roots and skipping file chunks.
func mfsEntityRootsProvider(mfsRoot *mfs.Root, bs blockstore.Blockstore, tracker walker.VisitedTracker) provider.KeyChanFunc {
	walk := func(ctx context.Context, root cid.Cid, emit func(cid.Cid) bool, opts ...walker.Option) error {
		return walker.WalkEntityRoots(ctx, root, walker.NodeFetcherFromBlockstore(bs), emit, opts...)
	}
	return mfsWalkProvider(mfsRoot, bs, tracker, walk)
}

// mfsWalkProvider builds a KeyChanFunc that flushes MFS, then walks
// with the given walkFunc using a shared tracker and locality check.
func mfsWalkProvider(mfsRoot *mfs.Root, bs blockstore.Blockstore, tracker walker.VisitedTracker, walk walkFunc) provider.KeyChanFunc {
	return func(ctx context.Context) (<-chan cid.Cid, error) {
		if err := mfsRoot.FlushMemFree(ctx); err != nil {
			return nil, fmt.Errorf("provider: error flushing MFS: %w", err)
		}
		rootNode, err := mfsRoot.GetDirectory().GetNode()
		if err != nil {
			return nil, fmt.Errorf("provider: error loading MFS root: %w", err)
		}

		ch := make(chan cid.Cid)
		go func() {
			defer close(ch)
			locality := func(ctx context.Context, c cid.Cid) (bool, error) {
				return bs.Has(ctx, c)
			}
			_ = walk(ctx, rootNode.Cid(), func(c cid.Cid) bool {
				select {
				case ch <- c:
					return true
				case <-ctx.Done():
					return false
				}
			}, walker.WithVisitedTracker(tracker), walker.WithLocality(locality))
		}()
		return ch, nil
	}
}

// createKeyProvider creates the appropriate KeyChanFunc based on strategy.
// fpRate is the bloom filter target false-positive rate (1/N) used by
// +unique and +entities cycles. Ignored by other strategies.
func createKeyProvider(strategyFlag config.ProvideStrategy, fpRate uint, in provStrategyIn) provider.KeyChanFunc {
	// +unique modifier: use bloom filter cross-DAG dedup
	useUnique := strategyFlag&config.ProvideStrategyUnique != 0
	if useUnique {
		basePinned := strategyFlag&config.ProvideStrategyPinned != 0
		baseMFS := strategyFlag&config.ProvideStrategyMFS != 0
		ds := in.Repo.Datastore()

		// return a KeyChanFunc that creates a fresh bloom each cycle
		return func(ctx context.Context) (<-chan cid.Cid, error) {
			count := readLastUniqueCount(ds)
			// size the bloom from the previous cycle's count (with growth
			// margin for repo changes between cycles), falling back to
			// DefaultBloomInitialCapacity on the very first cycle. The
			// bloom chain auto-grows if the repo exceeds this estimate.
			expectedItems := max(
				uint64(walker.DefaultBloomInitialCapacity),
				uint64(float64(count)*walker.BloomGrowthMargin),
			)
			// the tracker is shared across all sub-walks (MFS, recursive
			// pins, direct pins) within a single reprovide cycle. it
			// detects duplicate sub-DAG branches across recursive pins
			// that share content (e.g. append-only datasets where each
			// version differs by a small delta). when a CID is already
			// in the bloom, its entire subtree is skipped, reducing
			// traversal from O(pins * total_blocks) to O(unique_blocks).
			tracker, err := walker.NewBloomTracker(uint(expectedItems), fpRate)
			if err != nil {
				return nil, fmt.Errorf("bloom tracker: %w", err)
			}

			useEntities := strategyFlag&config.ProvideStrategyEntities != 0

			// select provider functions based on +entities modifier:
			// +entities uses WalkEntityRoots (skips file chunks),
			// +unique without +entities uses WalkDAG (all blocks).
			makePinProv := dspinner.NewUniquePinnedProvider
			makeMFSProv := uniqueMFSProvider
			if useEntities {
				makePinProv = dspinner.NewPinnedEntityRootsProvider
				makeMFSProv = mfsEntityRootsProvider
			}

			var inner provider.KeyChanFunc
			switch {
			case basePinned && baseMFS:
				// MFS first: walk MFS (locality-filtered), then pinned.
				// NewConcatProvider (not NewPrioritizedProvider) because
				// the shared bloom tracker already guarantees each CID
				// is emitted at most once -- no need for a second dedup
				// layer. NewBufferedProvider decouples the pinned
				// provider so the pinner lock is released promptly.
				inner = provider.NewConcatProvider(
					makeMFSProv(in.MFSRoot, in.Blockstore, tracker),
					provider.NewBufferedProvider(
						makePinProv(in.Pinner, in.Blockstore, tracker)),
				)
			case basePinned:
				inner = provider.NewBufferedProvider(
					makePinProv(in.Pinner, in.Blockstore, tracker))
			case baseMFS:
				inner = makeMFSProv(in.MFSRoot, in.Blockstore, tracker)
			default:
				return nil, fmt.Errorf("provider: +unique requires pinned and/or mfs")
			}

			// wrap inner channel to persist bloom count on successful close
			innerCh, err := inner(ctx)
			if err != nil {
				return nil, err
			}

			ch := make(chan cid.Cid)
			go func() {
				defer func() {
					if ctx.Err() == nil {
						persistUniqueCount(ds, tracker.Count())
					}
					providerLog.Infow("unique reprovide cycle finished",
						"providedCIDs", tracker.Count(),
						"skippedBranches", tracker.Deduplicated())
					close(ch)
				}()
				for c := range innerCh {
					select {
					case ch <- c:
					case <-ctx.Done():
						return
					}
				}
			}()

			providerLog.Infow("unique reprovide cycle started",
				"expectedItems", expectedItems,
				"previousCount", count,
			)
			return ch, nil
		}
	}

	// non-unique strategies (unchanged)
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
		providerLog.Error("cannot read previous reprovide strategy", "err", err)
		return
	}

	if !changed {
		return
	}

	providerLog.Infow("Provide.Strategy changed, clearing provide queue", "previous", previous, "current", strategy)
	provider.Clear()

	if err := persistStrategy(ctx, strategy, ds); err != nil {
		providerLog.Error("cannot update reprovide strategy", "err", err)
	}
}

func setReproviderKeyProvider(strategy string, fpRate uint) func(in provStrategyIn) provStrategyOut {
	strategyFlag := config.MustParseProvideStrategy(strategy)

	return func(in provStrategyIn) provStrategyOut {
		// Create the appropriate key provider based on strategy
		kcf := createKeyProvider(strategyFlag, fpRate, in)
		return provStrategyOut{
			ProvidingStrategy:    strategyFlag,
			ProvidingKeyChanFunc: kcf,
		}
	}
}
