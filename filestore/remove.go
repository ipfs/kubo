package filestore

import (
	"fmt"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	u "github.com/ipfs/go-ipfs/blocks/blockstore/util"
	"github.com/ipfs/go-ipfs/pin"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
)

// RmBlocks removes blocks from either the filestore or the
// blockstore.  It is similar to blockstore_util.RmBlocks but allows
// the removal of pinned block from one store if it is also in the
// other.
func RmBlocks(fs *Filestore, lock bs.GCLocker, pins pin.Pinner, cids []*cid.Cid, opts u.RmBlocksOpts) (<-chan interface{}, error) {
	// make the channel large enough to hold any result to avoid
	// blocking while holding the GCLock
	out := make(chan interface{}, len(cids))

	var blocks deleter
	switch opts.Prefix {
	case FilestorePrefix.String():
		blocks = fs.fm
	case bs.BlockPrefix.String():
		blocks = fs.bs
	default:
		return nil, fmt.Errorf("unknown prefix: %s", opts.Prefix)
	}

	go func() {
		defer close(out)

		unlocker := lock.GCLock()
		defer unlocker.Unlock()

		stillOkay := filterPinned(fs, pins, out, cids, blocks)

		for _, c := range stillOkay {
			err := blocks.DeleteBlock(c)
			if err != nil && opts.Force && (err == bs.ErrNotFound || err == ds.ErrNotFound) {
				// ignore non-existent blocks
			} else if err != nil {
				out <- &u.RemovedBlock{Hash: c.String(), Error: err.Error()}
			} else if !opts.Quiet {
				out <- &u.RemovedBlock{Hash: c.String()}
			}
		}
	}()
	return out, nil
}

type deleter interface {
	DeleteBlock(c *cid.Cid) error
}

func filterPinned(fs *Filestore, pins pin.Pinner, out chan<- interface{}, cids []*cid.Cid, foundIn deleter) []*cid.Cid {
	stillOkay := make([]*cid.Cid, 0, len(cids))
	res, err := pins.CheckIfPinned(cids...)
	if err != nil {
		out <- &u.RemovedBlock{Error: fmt.Sprintf("pin check failed: %s", err)}
		return nil
	}
	for _, r := range res {
		if !r.Pinned() || availableElsewhere(fs, foundIn, r.Key) {
			stillOkay = append(stillOkay, r.Key)
		} else {
			out <- &u.RemovedBlock{
				Hash:  r.Key.String(),
				Error: r.String(),
			}
		}
	}
	return stillOkay
}

func availableElsewhere(fs *Filestore, foundIn deleter, c *cid.Cid) bool {
	switch {
	case fs.fm == foundIn:
		have, _ := fs.bs.Has(c)
		return have
	case fs.bs == foundIn:
		have, _ := fs.fm.Has(c)
		return have
	default:
		// programmer error
		panic("invalid pointer for foundIn")
	}
}
