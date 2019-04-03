package node

import (
	"context"
	"os"
	"syscall"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/retrystore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	config "github.com/ipfs/go-ipfs-config"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/thirdparty/cidv0v1"
	"github.com/ipfs/go-ipfs/thirdparty/verifbs"
)

func isTooManyFDError(err error) bool {
	perr, ok := err.(*os.PathError)
	if ok && perr.Err == syscall.EMFILE {
		return true
	}

	return false
}

func RepoConfig(repo repo.Repo) (*config.Config, error) {
	return repo.Config()
}

func DatastoreCtor(repo repo.Repo) datastore.Datastore {
	return repo.Datastore()
}

type BaseBlocks blockstore.Blockstore

func BaseBlockstoreCtor(permanent bool, nilRepo bool) func(mctx MetricsCtx, repo repo.Repo, cfg *config.Config, lc fx.Lifecycle) (bs BaseBlocks, err error) {
	return func(mctx MetricsCtx, repo repo.Repo, cfg *config.Config, lc fx.Lifecycle) (bs BaseBlocks, err error) {
		rds := &retrystore.Datastore{
			Batching:    repo.Datastore(),
			Delay:       time.Millisecond * 200,
			Retries:     6,
			TempErrFunc: isTooManyFDError,
		}
		// hash security
		bs = blockstore.NewBlockstore(rds)
		bs = &verifbs.VerifBS{Blockstore: bs}

		opts := blockstore.DefaultCacheOpts()
		opts.HasBloomFilterSize = cfg.Datastore.BloomFilterSize
		if !permanent {
			opts.HasBloomFilterSize = 0
		}

		if !nilRepo {
			ctx, cancel := context.WithCancel(mctx)

			lc.Append(fx.Hook{
				OnStop: func(context context.Context) error {
					cancel()
					return nil
				},
			})
			bs, err = blockstore.CachedBlockstore(ctx, bs, opts)
			if err != nil {
				return nil, err
			}
		}

		bs = blockstore.NewIdStore(bs)
		bs = cidv0v1.NewBlockstore(bs)

		if cfg.Datastore.HashOnRead { // TODO: review: this is how it was done originally, is there a reason we can't just pass this directly?
			bs.HashOnRead(true)
		}

		return
	}
}

func GcBlockstoreCtor(repo repo.Repo, bb BaseBlocks, cfg *config.Config) (gclocker blockstore.GCLocker, gcbs blockstore.GCBlockstore, bs blockstore.Blockstore, fstore *filestore.Filestore) {
	gclocker = blockstore.NewGCLocker()
	gcbs = blockstore.NewGCBlockstore(bb, gclocker)

	if cfg.Experimental.FilestoreEnabled || cfg.Experimental.UrlstoreEnabled {
		// hash security
		fstore = filestore.NewFilestore(bb, repo.FileManager()) // TODO: mark optional
		gcbs = blockstore.NewGCBlockstore(fstore, gclocker)
		gcbs = &verifbs.VerifBSGC{GCBlockstore: gcbs}
	}
	bs = gcbs
	return
}
