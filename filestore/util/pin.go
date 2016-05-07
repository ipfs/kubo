package filestore_util

import (
	"errors"
	"fmt"
	"io"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	bk "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/pin"
	//context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

type ToFix struct {
	key  bk.Key
	good []bk.Key
}

func RepairPins(n *core.IpfsNode, fs *Datastore, wtr io.Writer, dryRun bool) error {
	pinning := n.Pinning
	bs := n.Blockstore
	rm_list := make([]bk.Key, 0)
	for _, k := range pinning.DirectKeys() {
		exists, err := bs.Has(k)
		if err != nil {
			return err
		}
		if !exists {
			rm_list = append(rm_list, k)
		}
	}

	rm_list_rec := make([]bk.Key, 0)
	fix_list := make([]ToFix, 0)
	for _, k := range pinning.RecursiveKeys() {
		exists, err := bs.Has(k)
		if err != nil {
			return err
		}
		if !exists {
			rm_list_rec = append(rm_list_rec, k)
		}
		good := make([]bk.Key, 0)
		ok, err := verifyRecPin(k, &good, fs, bs)
		if err != nil {
			return err
		}
		if ok {
			// all okay, keep pin
		} else {
			fix_list = append(fix_list, ToFix{k, good})
		}
	}

	for _, key := range rm_list {
		if dryRun {
			fmt.Fprintf(wtr, "Will remove direct pin %s\n", key)
		} else {
			fmt.Fprintf(wtr, "Removing direct pin %s\n", key)
			pinning.RemovePinWithMode(key, pin.Direct)
		}
	}
	for _, key := range rm_list_rec {
		if dryRun {
			fmt.Fprintf(wtr, "Will remove recursive pin %s\n", key)
		} else {
			fmt.Fprintf(wtr, "Removing recursive pin %s\n", key)
			pinning.RemovePinWithMode(key, pin.Recursive)
		}
	}
	for _, to_fix := range fix_list {
		if dryRun {
			fmt.Fprintf(wtr, "Will repair recursive pin %s by:\n", to_fix.key)
			for _, key := range to_fix.good {
				fmt.Fprintf(wtr, "  adding pin %s\n", key)
			}
			fmt.Fprintf(wtr, "  and converting %s to a direct pin\n", to_fix.key)
		} else {
			fmt.Fprintf(wtr, "Repairing recursive pin %s:\n", to_fix.key)
			for _, key := range to_fix.good {
				fmt.Fprintf(wtr, "  adding pin %s\n", key)
				pinning.RemovePinWithMode(key, pin.Direct)
				pinning.PinWithMode(key, pin.Recursive)
			}
			fmt.Fprintf(wtr, "  converting %s to a direct pin\n", to_fix.key)
			pinning.RemovePinWithMode(to_fix.key, pin.Recursive)
			pinning.PinWithMode(to_fix.key, pin.Direct)
		}
	}
	if !dryRun {
		err := pinning.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}

// verify a key and build up a list of good children
// if the key is okay add itself to the good list and return true
// if some of the children are missing add the non-missing children and return false
// if an error return it
func verifyRecPin(key bk.Key, good *[]bk.Key, fs *Datastore, bs b.Blockstore) (bool, error) {
	n, _, status := getNode(key.DsKey(), key, fs, bs)
	if status == StatusKeyNotFound {
		return false, nil
	} else if AnError(status) {
		return false, errors.New("Error when retrieving key")
	} else if n == nil {
		// A unchecked leaf
		*good = append(*good, key)
		return true, nil
	}
	allOk := true
	goodChildren := make([]bk.Key, 0)
	for _, link := range n.Links {
		key := bk.Key(link.Hash)
		ok, err := verifyRecPin(key, &goodChildren, fs, bs)
		if err != nil {
			return false, err
		} else if !ok {
			allOk = false
		}
	}
	if allOk {
		*good = append(*good, key)
		return true, nil
	} else {
		*good = append(*good, goodChildren...)
		return false, nil
	}
}

