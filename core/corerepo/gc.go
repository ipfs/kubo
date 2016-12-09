package corerepo

import (
	"context"
	"errors"
	"time"

	"github.com/ipfs/go-ipfs/core"
	mfs "github.com/ipfs/go-ipfs/mfs"
	gc "github.com/ipfs/go-ipfs/pin/gc"
	repo "github.com/ipfs/go-ipfs/repo"

	humanize "gx/ipfs/QmPSBJL4momYnE7DcUyk2DVhD6rH488ZmHBGLbxNdhU44K/go-humanize"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
)

var log = logging.Logger("corerepo")

var ErrMaxStorageExceeded = errors.New("Maximum storage limit exceeded. Maybe unpin some files?")

type KeyRemoved struct {
	Key *cid.Cid
}

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
		r.SetConfigKey("Datastore.StorageMax", "10GB")
		cfg.Datastore.StorageMax = "10GB"
	}
	if cfg.Datastore.StorageGCWatermark == 0 {
		r.SetConfigKey("Datastore.StorageGCWatermark", 90)
		cfg.Datastore.StorageGCWatermark = 90
	}

	storageMax, err := humanize.ParseBytes(cfg.Datastore.StorageMax)
	if err != nil {
		return nil, err
	}
	storageGC := storageMax * uint64(cfg.Datastore.StorageGCWatermark) / 100

	// calculate the slack space between StorageMax and StorageGCWatermark
	// used to limit GC duration
	slackGB := (storageMax - storageGC) / 10e9
	if slackGB < 1 {
		slackGB = 1
	}

	return &GC{
		Node:       n,
		Repo:       r,
		StorageMax: storageMax,
		StorageGC:  storageGC,
		SlackGB:    slackGB,
	}, nil
}

func BestEffortRoots(filesRoot *mfs.Root) ([]*cid.Cid, error) {
	rootDag, err := filesRoot.GetValue().GetNode()
	if err != nil {
		return nil, err
	}

	return []*cid.Cid{rootDag.Cid()}, nil
}

func GarbageCollect(n *core.IpfsNode, ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // in case error occurs during operation
	roots, err := BestEffortRoots(n.FilesRoot)
	if err != nil {
		return err
	}
	rmed, err := gc.GC(ctx, n.Blockstore, n.DAG, n.Pinning, roots)
	if err != nil {
		return err
	}

	for {
		select {
		case _, ok := <-rmed:
			if !ok {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

func GarbageCollectAsync(n *core.IpfsNode, ctx context.Context) (<-chan *KeyRemoved, error) {
	roots, err := BestEffortRoots(n.FilesRoot)
	if err != nil {
		return nil, err
	}
	rmed, err := gc.GC(ctx, n.Blockstore, n.DAG, n.Pinning, roots)
	if err != nil {
		return nil, err
	}

	out := make(chan *KeyRemoved)
	go func() {
		defer close(out)
		for k := range rmed {
			select {
			case out <- &KeyRemoved{k}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
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
	storage, err := gc.Repo.GetStorageUsage()
	if err != nil {
		return err
	}

	if storage+offset > gc.StorageGC {
		if storage+offset > gc.StorageMax {
			log.Warningf("pre-GC: %s", ErrMaxStorageExceeded)
		}

		// Do GC here
		log.Info("Watermark exceeded. Starting repo GC...")
		defer log.EventBegin(ctx, "repoGC").Done()

		if err := GarbageCollect(gc.Node, ctx); err != nil {
			return err
		}
		log.Infof("Repo GC done. See `ipfs repo stat` to see how much space got freed.\n")
	}
	return nil
}
