package corerepo

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/gc"
	"github.com/ipfs/kubo/repo"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("corerepo")

var ErrMaxStorageExceeded = errors.New("maximum storage limit exceeded. Try to unpin some files")

type GC struct {
	Node       *core.IpfsNode
	Repo       repo.Repo
	StorageMax uint64
	StorageGC  uint64
	SlackGB    uint64
	Storage    uint64
}

func NewGC(n *core.IpfsNode) (*GC, error) {
	r := n.Repo
	cfg, err := r.Config()
	if err != nil {
		return nil, err
	}

	// check if cfg has these fields initialized
	// TODO: there should be a general check for all of the cfg fields
	// maybe distinguish between user config file and default struct?
	if cfg.Datastore.StorageMax == "" {
		if err := r.SetConfigKey("Datastore.StorageMax", "10GB"); err != nil {
			return nil, err
		}
		cfg.Datastore.StorageMax = "10GB"
	}
	if cfg.Datastore.StorageGCWatermark == 0 {
		if err := r.SetConfigKey("Datastore.StorageGCWatermark", 90); err != nil {
			return nil, err
		}
		cfg.Datastore.StorageGCWatermark = 90
	}

	storageMax, err := humanize.ParseBytes(cfg.Datastore.StorageMax)
	if err != nil {
		return nil, err
	}
	storageGC := storageMax * uint64(cfg.Datastore.StorageGCWatermark) / 100

	// calculate the slack space between StorageMax and StorageGCWatermark
	// used to limit GC duration
	slackGB := max((storageMax-storageGC)/10e9, 1)

	return &GC{
		Node:       n,
		Repo:       r,
		StorageMax: storageMax,
		StorageGC:  storageGC,
		SlackGB:    slackGB,
	}, nil
}

// BestEffortRoots returns the CIDs of the live MFS roots GC must keep: the
// node's FilesRoot plus any extra roots registered via
// IpfsNode.RegisterMFSRoot (for example the per-key roots of a writable /ipns
// FUSE mount). Extra roots are walked best-effort: one that cannot be read
// (e.g. because its mount is unmounting) is logged and skipped rather than
// failing the whole GC. FilesRoot is required.
func BestEffortRoots(n *core.IpfsNode) ([]cid.Cid, error) {
	rootDag, err := n.FilesRoot.GetDirectory().GetNode()
	if err != nil {
		return nil, err
	}
	roots := []cid.Cid{rootDag.Cid()}

	for _, r := range n.MFSRoots() {
		if r == nil {
			continue
		}
		nd, err := r.GetDirectory().GetNode()
		if err != nil {
			log.Warnf("gc: skipping unreadable MFS root (mount unmounting?): %s", err)
			continue
		}
		roots = append(roots, nd.Cid())
	}

	return roots, nil
}

func GarbageCollect(n *core.IpfsNode, ctx context.Context) error {
	rmed := gc.GC(ctx, n.Blockstore, n.Repo.Datastore(), n.Pinning, func(context.Context) ([]cid.Cid, error) {
		return BestEffortRoots(n)
	})

	return CollectResult(ctx, rmed, nil)
}

// CollectResult collects the output of a garbage collection run and calls the
// given callback for each object removed.  It also collects all errors into a
// MultiError which is returned after the gc is completed.
func CollectResult(ctx context.Context, gcOut <-chan gc.Result, cb func(cid.Cid)) error {
	var errors []error
loop:
	for {
		select {
		case res, ok := <-gcOut:
			if !ok {
				break loop
			}
			if res.Error != nil {
				errors = append(errors, res.Error)
			} else if res.KeyRemoved.Defined() && cb != nil {
				cb(res.KeyRemoved)
			}
		case <-ctx.Done():
			errors = append(errors, ctx.Err())
			break loop
		}
	}

	switch len(errors) {
	case 0:
		return nil
	case 1:
		return errors[0]
	default:
		return NewMultiError(errors...)
	}
}

// NewMultiError creates a new MultiError object from a given slice of errors.
func NewMultiError(errs ...error) *MultiError {
	return &MultiError{errs[:len(errs)-1], errs[len(errs)-1]}
}

// MultiError contains the results of multiple errors.
type MultiError struct {
	Errors  []error
	Summary error
}

func (e *MultiError) Error() string {
	var buf bytes.Buffer
	for _, err := range e.Errors {
		buf.WriteString(err.Error())
		buf.WriteString("; ")
	}
	buf.WriteString(e.Summary.Error())
	return buf.String()
}

func GarbageCollectAsync(n *core.IpfsNode, ctx context.Context) <-chan gc.Result {
	return gc.GC(ctx, n.Blockstore, n.Repo.Datastore(), n.Pinning, func(context.Context) ([]cid.Cid, error) {
		return BestEffortRoots(n)
	})
}

func PeriodicGC(ctx context.Context, node *core.IpfsNode) error {
	cfg, err := node.Repo.Config()
	if err != nil {
		return err
	}

	if cfg.Datastore.GCPeriod == "" {
		cfg.Datastore.GCPeriod = "1h"
	}

	period, err := time.ParseDuration(cfg.Datastore.GCPeriod)
	if err != nil {
		return err
	}
	if int64(period) == 0 {
		// if duration is 0, it means GC is disabled.
		return nil
	}

	gc, err := NewGC(node)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(period):
			// the private func maybeGC doesn't compute storageMax, storageGC, slackGC so that they are not re-computed for every cycle
			if err := gc.maybeGC(ctx, 0); err != nil {
				log.Error(err)
			}
		}
	}
}

func ConditionalGC(ctx context.Context, node *core.IpfsNode, offset uint64) error {
	gc, err := NewGC(node)
	if err != nil {
		return err
	}
	return gc.maybeGC(ctx, offset)
}

func (gc *GC) maybeGC(ctx context.Context, offset uint64) error {
	storage, err := gc.Repo.GetStorageUsage(ctx)
	if err != nil {
		return err
	}

	if storage+offset > gc.StorageGC {
		if storage+offset > gc.StorageMax {
			log.Warnf("pre-GC: %s", ErrMaxStorageExceeded)
		}

		// Do GC here
		log.Info("Watermark exceeded. Starting repo GC...")

		if err := GarbageCollect(gc.Node, ctx); err != nil {
			return err
		}
		log.Infof("Repo GC done. See `ipfs repo stat` to see how much space got freed.\n")
	}
	return nil
}
