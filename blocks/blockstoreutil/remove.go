// Package blockstoreutil provides utility functions for Blockstores.
package blockstoreutil

import (
	"context"
	"errors"
	"fmt"

	bs "github.com/ipfs/boxo/blockstore"
	pin "github.com/ipfs/boxo/pinning/pinner"
	cid "github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
)

// RemovedBlock is used to represent the result of removing a block.
// If a block was removed successfully, then the Error will be empty.
// If a block could not be removed, then Error will contain the
// reason the block could not be removed.  If the removal was aborted
// due to a fatal error, Hash will be empty, Error will contain the
// reason, and no more results will be sent.
type RemovedBlock struct {
	Hash  string
	Error error
}

// RmBlocksOpts is used to wrap options for RmBlocks().
type RmBlocksOpts struct {
	Prefix string
	Quiet  bool
	Force  bool
}

// RmBlocks removes the blocks provided in the cids slice.
// It returns a channel where objects of type RemovedBlock are placed, when
// not using the Quiet option. Block removal is asynchronous and will
// skip any pinned blocks.
func RmBlocks(ctx context.Context, blocks bs.GCBlockstore, pins pin.Pinner, cids []cid.Cid, opts RmBlocksOpts) (<-chan interface{}, error) {
	// make the channel large enough to hold any result to avoid
	// blocking while holding the GCLock
	out := make(chan interface{}, len(cids))
	go func() {
		defer close(out)

		unlocker := blocks.GCLock(ctx)
		defer unlocker.Unlock(ctx)

		stillOkay := FilterPinned(ctx, pins, out, cids)

		for _, c := range stillOkay {
			// Kept for backwards compatibility. We may want to
			// remove this sometime in the future.
			has, err := blocks.Has(ctx, c)
			if err != nil {
				out <- &RemovedBlock{Hash: c.String(), Error: err}
				continue
			}
			if !has && !opts.Force {
				out <- &RemovedBlock{Hash: c.String(), Error: format.ErrNotFound{Cid: c}}
				continue
			}

			err = blocks.DeleteBlock(ctx, c)
			if err != nil {
				out <- &RemovedBlock{Hash: c.String(), Error: err}
			} else if !opts.Quiet {
				out <- &RemovedBlock{Hash: c.String()}
			}
		}
	}()
	return out, nil
}

// FilterPinned takes a slice of Cids and returns it with the pinned Cids
// removed. If a Cid is pinned, it will place RemovedBlock objects in the given
// out channel, with an error which indicates that the Cid is pinned.
// This function is used in RmBlocks to filter out any blocks which are not
// to be removed (because they are pinned).
func FilterPinned(ctx context.Context, pins pin.Pinner, out chan<- interface{}, cids []cid.Cid) []cid.Cid {
	stillOkay := make([]cid.Cid, 0, len(cids))
	res, err := pins.CheckIfPinned(ctx, cids...)
	if err != nil {
		out <- &RemovedBlock{Error: fmt.Errorf("pin check failed: %w", err)}
		return nil
	}
	for _, r := range res {
		if !r.Pinned() {
			stillOkay = append(stillOkay, r.Key)
		} else {
			out <- &RemovedBlock{
				Hash:  r.Key.String(),
				Error: errors.New(r.String()),
			}
		}
	}
	return stillOkay
}
