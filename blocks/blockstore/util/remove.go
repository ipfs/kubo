package blockstore_util

import (
	"fmt"
	"io"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	"github.com/ipfs/go-ipfs/pin"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
	cid "gx/ipfs/QmfSc2xehWmWLnwwYR91Y8QF4xdASypTFVknutoKQS3GHp/go-cid"
)

type RemovedBlock struct {
	Hash  string `json:",omitempty"`
	Error string `json:",omitempty"`
}

type RmBlocksOpts struct {
	Prefix string
	Quiet  bool
	Force  bool
}

func RmBlocks(blocks bs.GCBlockstore, pins pin.Pinner, out chan<- interface{}, cids []*cid.Cid, opts RmBlocksOpts) error {
	go func() {
		defer close(out)

		unlocker := blocks.GCLock()
		defer unlocker.Unlock()

		stillOkay, err := checkIfPinned(pins, cids, out)
		if err != nil {
			out <- &RemovedBlock{Error: fmt.Sprintf("pin check failed: %s", err)}
			return
		}

		for _, c := range stillOkay {
			err := blocks.DeleteBlock(key.Key(c.Hash()))
			if err != nil && opts.Force && (err == bs.ErrNotFound || err == ds.ErrNotFound) {
				// ignore non-existent blocks
			} else if err != nil {
				out <- &RemovedBlock{Hash: c.String(), Error: err.Error()}
			} else if !opts.Quiet {
				out <- &RemovedBlock{Hash: c.String()}
			}
		}
	}()
	return nil
}

func checkIfPinned(pins pin.Pinner, cids []*cid.Cid, out chan<- interface{}) ([]*cid.Cid, error) {
	stillOkay := make([]*cid.Cid, 0, len(cids))
	res, err := pins.CheckIfPinned(cids...)
	if err != nil {
		return nil, err
	}
	for _, r := range res {
		if !r.Pinned() {
			stillOkay = append(stillOkay, r.Key)
		} else {
			out <- &RemovedBlock{
				Hash:  r.Key.String(),
				Error: r.String(),
			}
		}
	}
	return stillOkay, nil
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
