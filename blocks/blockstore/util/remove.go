package blockstore_util

import (
	"fmt"
	"io"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/pin"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
)

// RemovedBlock is used to respresent the result of removing a block.
// If a block was removed successfully than the Error string will be
// empty.  If a block could not be removed than Error will contain the
// reason the block could not be removed.  If the removal was aborted
// due to a fatal error Hash will be be empty, Error will contain the
// reason, and no more results will be sent.
type RemovedBlock struct {
	Hash  string `json:",omitempty"`
	Error string `json:",omitempty"`
}

type RmBlocksOpts struct {
	Prefix string
	Quiet  bool
	Force  bool
}

func RmBlocks(mbs bs.MultiBlockstore, pins pin.Pinner, out chan<- interface{}, keys []key.Key, opts RmBlocksOpts) error {
	prefix := opts.Prefix
	if prefix == "" {
		prefix = mbs.Mounts()[0]
	}
	blocks := mbs.Mount(prefix)
	if blocks == nil {
		return fmt.Errorf("Could not find blockstore: %s\n", prefix)
	}

	go func() {
		defer close(out)

		unlocker := mbs.GCLock()
		defer unlocker.Unlock()

		stillOkay := FilterPinned(mbs, pins, out, keys, prefix)

		for _, k := range stillOkay {
			err := blocks.DeleteBlock(k)
			if err != nil && opts.Force && (err == bs.ErrNotFound || err == ds.ErrNotFound) {
				// ignore non-existent blocks
			} else if err != nil {
				out <- &RemovedBlock{Hash: k.String(), Error: err.Error()}
			} else if !opts.Quiet {
				out <- &RemovedBlock{Hash: k.String()}
			}
		}
	}()
	return nil
}

func FilterPinned(mbs bs.MultiBlockstore, pins pin.Pinner, out chan<- interface{}, keys []key.Key, prefix string) []key.Key {
	stillOkay := make([]key.Key, 0, len(keys))
	res, err := pins.CheckIfPinned(keys...)
	if err != nil {
		out <- &RemovedBlock{Error: fmt.Sprintf("pin check failed: %s", err)}
		return nil
	}
	for _, r := range res {
		if !r.Pinned() || AvailableElsewhere(mbs, prefix, r.Key) {
			stillOkay = append(stillOkay, r.Key)
		} else {
			out <- &RemovedBlock{
				Hash:  r.Key.String(),
				Error: r.String(),
			}
		}
	}
	return stillOkay
}

func AvailableElsewhere(mbs bs.MultiBlockstore, prefix string, key key.Key) bool {
	locations := mbs.Locate(key)
	for _, loc := range locations {
		if loc.Error == nil && loc.Prefix != prefix {
			return true
		}
	}
	return false
}

func ProcRmOutput(in <-chan interface{}, sout io.Writer, serr io.Writer) error {
	someFailed := false
	for res := range in {
		r := res.(*RemovedBlock)
		if r.Hash == "" && r.Error != "" {
			return fmt.Errorf("aborted: %s", r.Error)
		} else if r.Error != "" {
			someFailed = true
			fmt.Fprintf(serr, "cannot remove %s: %s\n", r.Hash, r.Error)
		} else {
			fmt.Fprintf(sout, "removed %s\n", r.Hash)
		}
	}
	if someFailed {
		return fmt.Errorf("some blocks not removed")
	}
	return nil
}
