package node

import (
	"bytes"
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	blockstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	config "github.com/ipfs/kubo/config"
	"go.uber.org/fx"

	"github.com/ipfs/boxo/filestore"
	"github.com/ipfs/boxo/provider"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/thirdparty/verifbs"
)

// RepoConfig loads configuration from the repo
func RepoConfig(repo repo.Repo) (*config.Config, error) {
	cfg, err := repo.Config()
	return cfg, err
}

// Datastore provides the datastore
func Datastore(repo repo.Repo) datastore.Datastore {
	return repo.Datastore()
}

// BaseBlocks is the lower level blockstore without GC or Filestore layers
type BaseBlocks blockstore.Blockstore

// BaseBlockstoreCtor creates cached blockstore backed by the provided datastore
func BaseBlockstoreCtor(
	cacheOpts blockstore.CacheOpts,
	hashOnRead bool,
	writeThrough bool,
	providingStrategy string,
) func(mctx helpers.MetricsCtx, repo repo.Repo, prov DHTProvider, lc fx.Lifecycle) (bs BaseBlocks, err error) {
	return func(mctx helpers.MetricsCtx, repo repo.Repo, prov DHTProvider, lc fx.Lifecycle) (bs BaseBlocks, err error) {
		opts := []blockstore.Option{blockstore.WriteThrough(writeThrough)}

		// Blockstore providing integration:
		// When strategy includes "all" the blockstore directly provides blocks as they're Put.
		// Important: Provide calls from blockstore are intentionally BLOCKING.
		// The Provider implementation (not the blockstore) should handle concurrency/queuing.
		// This avoids spawning unbounded goroutines for concurrent block additions.
		strategyFlag := config.MustParseProvideStrategy(providingStrategy)
		if strategyFlag&config.ProvideStrategyAll != 0 {
			opts = append(opts, blockstore.Provider(prov))
		}

		// hash security
		bs = blockstore.NewBlockstore(
			repo.Datastore(),
			opts...,
		)
		bs = &verifbs.VerifBS{Blockstore: bs}
		bs, err = blockstore.CachedBlockstore(helpers.LifecycleCtx(mctx, lc), bs, cacheOpts)
		if err != nil {
			return nil, err
		}

		bs = blockstore.NewIdStore(bs)

		if hashOnRead {
			bs = &blockstore.ValidatingBlockstore{Blockstore: bs}
		}

		return
	}
}

// GcBlockstoreCtor wraps the base blockstore with GC and Filestore layers
func GcBlockstoreCtor(bb BaseBlocks) (gclocker blockstore.GCLocker, gcbs blockstore.GCBlockstore, bs blockstore.Blockstore) {
	gclocker = blockstore.NewGCLocker()
	gcbs = blockstore.NewGCBlockstore(bb, gclocker)

	bs = gcbs
	return
}

// FilestoreBlockstoreCtor wraps GcBlockstore and adds Filestore support
func FilestoreBlockstoreCtor(
	providingStrategy string,
) func(repo repo.Repo, bb BaseBlocks, prov DHTProvider) (gclocker blockstore.GCLocker, gcbs blockstore.GCBlockstore, bs blockstore.Blockstore, fstore *filestore.Filestore) {
	return func(repo repo.Repo, bb BaseBlocks, prov DHTProvider) (gclocker blockstore.GCLocker, gcbs blockstore.GCBlockstore, bs blockstore.Blockstore, fstore *filestore.Filestore) {
		gclocker = blockstore.NewGCLocker()

		var fstoreProv provider.MultihashProvider
		strategyFlag := config.MustParseProvideStrategy(providingStrategy)
		if strategyFlag&config.ProvideStrategyAll != 0 {
			fstoreProv = prov
		}

		fstore = filestore.NewFilestore(bb, repo.FileManager(), fstoreProv)

		// hash security
		gcbs = blockstore.NewGCBlockstore(fstore, gclocker)
		gcbs = &verifbs.VerifBSGC{GCBlockstore: gcbs}

		bs = gcbs
		return
	}
}

// dhtValuePurgeDoneKey marks that stale pre-v0.42.0 go-libp2p-kad-dht value
// records were purged from the repo datastore, so the scan runs only once.
var dhtValuePurgeDoneKey = datastore.NewKey("/local/dht-values-purged")

// oldDHTValueBase32 is the encoding go-libp2p-kad-dht used before v0.42.0 to
// derive datastore keys for value records: uppercase base32 without padding
// of the whole routing key, stored as a single component at the datastore
// root (e.g. "/F5UXA..." for an "/ipns/..." routing key).
var oldDHTValueBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// oldDHTValueNamespaces are the routing key namespaces of Kubo's DHT record
// validator. Only records under these namespaces were ever stored.
var oldDHTValueNamespaces = [][]byte{[]byte("/pk/"), []byte("/ipns/")}

// PurgeStaleDHTValueRecords removes DHT value records written by
// go-libp2p-kad-dht before v0.42.0 from the repo datastore. Since v0.42.0
// value records live under namespaced keys (in Kubo "/dht/pk/...",
// "/dht/ipns/...", see irouting.DHTDatastoreKey), so the old root-level
// records are unreachable orphans: nothing reads them, and the
// old lazy expiry (delete on read of the same key) can never trigger, so they
// would stay on disk forever. They are not migrated to the new keys because
// value records expire after MaxRecordAge (48h by default) anyway and
// publishers rewrite them far more often than that.
//
// The purge must scan the whole datastore, including the blockstore mount:
// the old keys are single components at the datastore root ("/<base32>"),
// and Query.Prefix matches whole path components only ("/F5" finds "/F5/x"
// but never "/F5UX..."), so no prefix short of "/" can reach them. It
// therefore runs once, in the background, and records completion under
// dhtValuePurgeDoneKey so later daemon starts skip the scan with a single
// Get. Rescanning on every start would repay the full walk for keys
// that post-v0.42.0 code can never write again. An interrupted purge resumes
// on the next start; deletions are idempotent.
//
// Caveat: a downgrade to Kubo <= 0.42 can write old-format records again, and
// with the marker already set they will not be purged on re-upgrade. Such
// leftovers are bounded and harmless; delete the marker key to force a
// re-scan.
func PurgeStaleDHTValueRecords(lc fx.Lifecycle, repo repo.Repo) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(done)
				purgeStaleDHTValueRecords(ctx, repo.Datastore())
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			<-done
			return nil
		},
	})
}

func purgeStaleDHTValueRecords(ctx context.Context, ds datastore.Batching) {
	switch _, err := ds.Get(ctx, dhtValuePurgeDoneKey); {
	case err == nil:
		return // already purged
	case !errors.Is(err, datastore.ErrNotFound):
		if ctx.Err() == nil {
			logger.Warnw("checking DHT value purge marker failed", "error", err)
		}
		return
	}

	count, err := deleteStaleDHTValueRecords(ctx, ds)
	if err != nil {
		if ctx.Err() != nil {
			logger.Infow("purge of stale DHT value records interrupted by shutdown, will resume on next start")
		} else {
			logger.Warnw("purge of stale DHT value records failed, will retry on next start", "error", err)
		}
		return
	}
	if err := ds.Put(ctx, dhtValuePurgeDoneKey, []byte{1}); err != nil {
		logger.Warnw("writing DHT value purge marker failed", "error", err)
		return
	}
	if err := ds.Sync(ctx, dhtValuePurgeDoneKey); err != nil {
		logger.Warnw("syncing DHT value purge marker failed", "error", err)
		return
	}
	if count > 0 {
		logger.Infow("purged stale DHT value records from datastore", "keys", count)
	}
}

// deleteStaleDHTValueRecords scans the whole datastore (keys only) and
// deletes old-format DHT value records in batches. It returns the number of
// deleted keys.
func deleteStaleDHTValueRecords(ctx context.Context, ds datastore.Batching) (int, error) {
	results, err := ds.Query(ctx, query.Query{Prefix: "/", KeysOnly: true})
	if err != nil {
		return 0, fmt.Errorf("querying datastore for stale DHT value records: %w", err)
	}
	defer results.Close()

	syncKey := datastore.NewKey("/")
	var batch datastore.Batch
	var count, pending int
	for result := range results.Next() {
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
		if result.Error != nil {
			return count, fmt.Errorf("iterating datastore keys: %w", result.Error)
		}
		if !isStaleDHTValueRecordKey(result.Key) {
			continue
		}
		if batch == nil {
			batch, err = ds.Batch(ctx)
			if err != nil {
				return count, fmt.Errorf("creating delete batch: %w", err)
			}
		}
		if err := batch.Delete(ctx, datastore.NewKey(result.Key)); err != nil {
			return count, fmt.Errorf("batch deleting stale key %s: %w", result.Key, err)
		}
		count++
		pending++
		if pending >= purgeBatchSize {
			if err := batch.Commit(ctx); err != nil {
				return count, fmt.Errorf("committing delete batch: %w", err)
			}
			if err := ds.Sync(ctx, syncKey); err != nil {
				return count, fmt.Errorf("syncing deletes: %w", err)
			}
			batch = nil
			pending = 0
		}
	}
	if pending > 0 {
		if err := batch.Commit(ctx); err != nil {
			return count, fmt.Errorf("committing delete batch: %w", err)
		}
		if err := ds.Sync(ctx, syncKey); err != nil {
			return count, fmt.Errorf("syncing deletes: %w", err)
		}
	}
	return count, nil
}

// isStaleDHTValueRecordKey reports whether key is a pre-v0.42.0 DHT value
// record: a single root-level path component that base32-decodes to a routing
// key in one of the DHT value namespaces. Everything else in the datastore
// (blocks, pins, MFS root, namespaced post-v0.42.0 records, IPNS publisher
// records, ...) lives under multi-component paths or does not decode to a
// namespaced routing key, so it never matches.
func isStaleDHTValueRecordKey(key string) bool {
	if len(key) < 2 || key[0] != '/' || strings.IndexByte(key[1:], '/') != -1 {
		return false
	}
	decoded, err := oldDHTValueBase32.DecodeString(key[1:])
	if err != nil {
		return false
	}
	for _, ns := range oldDHTValueNamespaces {
		if bytes.HasPrefix(decoded, ns) {
			return true
		}
	}
	return false
}
