package node

import (
	"context"
	"time"

	"github.com/dustin/go-humanize"
	blockstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/ipld/merkledag"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	peer "github.com/libp2p/go-libp2p/core/peer"
	routing "github.com/libp2p/go-libp2p/core/routing"
	"go.uber.org/fx"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/ondemandpin"
	"github.com/ipfs/kubo/repo"
)

// pinIdleTimeout cancels a pin when no new blocks arrive for this long.
const pinIdleTimeout = 2 * time.Minute

type kuboPinService struct {
	pinner pin.Pinner
	dag    format.DAGService
	bs     blockstore.GCBlockstore
}

func (s *kuboPinService) Pin(ctx context.Context, c cid.Cid, name string) error {
	defer s.bs.PinLock(ctx).Unlock(ctx)

	node, err := s.fetchRoot(ctx, c)
	if err != nil {
		return err
	}
	if err := s.pinDAG(ctx, node, name); err != nil {
		return err
	}
	return s.pinner.Flush(ctx)
}

func (s *kuboPinService) fetchRoot(ctx context.Context, c cid.Cid) (format.Node, error) {
	ctx, cancel := context.WithTimeout(ctx, pinIdleTimeout)
	defer cancel()
	return s.dag.Get(ctx, c)
}

// pinDAG recursively pins the DAG rooted at node.
// A background goroutine monitors block-fetching progress and cancels the operation if no new blocks arrive within pinIdleTimeout.
func (s *kuboPinService) pinDAG(ctx context.Context, node format.Node, name string) error {
	tracker := new(merkledag.ProgressTracker)
	trackerCtx := tracker.DeriveContext(ctx)
	pinCtx, cancel := context.WithCancel(trackerCtx)
	defer cancel()

	go watchPinProgress(pinCtx, cancel, tracker, pinIdleTimeout)
	return s.pinner.Pin(pinCtx, node, true, name)
}

func watchPinProgress(ctx context.Context, cancel context.CancelFunc, tracker *merkledag.ProgressTracker, timeout time.Duration) {
	var last int
	lastProgress := time.Now()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if cur := tracker.Value(); cur != last {
				last = cur
				lastProgress = time.Now()
			} else if time.Since(lastProgress) >= timeout {
				cancel()
				return
			}
		}
	}
}

func (s *kuboPinService) Unpin(ctx context.Context, c cid.Cid) error {
	defer s.bs.PinLock(ctx).Unlock(ctx)

	if err := s.pinner.Unpin(ctx, c, true); err != nil {
		return err
	}
	return s.pinner.Flush(ctx)
}

func (s *kuboPinService) IsPinned(ctx context.Context, c cid.Cid) (bool, error) {
	_, pinned, err := s.pinner.IsPinned(ctx, c)
	return pinned, err
}

func (s *kuboPinService) HasPinWithName(ctx context.Context, c cid.Cid, name string) (bool, error) {
	return ondemandpin.PinHasName(ctx, s.pinner, c, name)
}

type kuboStorageChecker struct {
	repo repo.Repo
}

func (s *kuboStorageChecker) StorageUsage(ctx context.Context) (uint64, uint64, error) {
	cfg, err := s.repo.Config()
	if err != nil {
		return 0, 0, err
	}
	used, err := s.repo.GetStorageUsage(ctx)
	if err != nil {
		return 0, 0, err
	}
	if cfg.Datastore.StorageMax == "" {
		return used, 0, nil
	}
	max, err := humanize.ParseBytes(cfg.Datastore.StorageMax)
	if err != nil {
		return 0, 0, err
	}
	wm := cfg.Datastore.StorageGCWatermark
	if wm <= 0 || wm > 100 {
		wm = 90
	}
	return used, max * uint64(wm) / 100, nil
}

func OnDemandPinStore(r repo.Repo) *ondemandpin.Store {
	return ondemandpin.NewStore(r.Datastore())
}

func OnDemandPinChecker(cfg config.OnDemandPinning) func(
	mctx helpers.MetricsCtx,
	lc fx.Lifecycle,
	r repo.Repo,
	store *ondemandpin.Store,
	pinner pin.Pinner,
	cr routing.ContentRouting,
	dag format.DAGService,
	bs blockstore.GCBlockstore,
	id peer.ID,
) *ondemandpin.Checker {
	return func(
		mctx helpers.MetricsCtx,
		lc fx.Lifecycle,
		r repo.Repo,
		store *ondemandpin.Store,
		pinner pin.Pinner,
		cr routing.ContentRouting,
		dag format.DAGService,
		bs blockstore.GCBlockstore,
		id peer.ID,
	) *ondemandpin.Checker {
		pins := &kuboPinService{pinner: pinner, dag: dag, bs: bs}
		storage := &kuboStorageChecker{repo: r}
		checker := ondemandpin.NewChecker(store, pins, storage, cr, id, cfg)
		ctx := helpers.LifecycleCtx(mctx, lc)

		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go checker.Run(ctx)
				return nil
			},
		})
		return checker
	}
}
