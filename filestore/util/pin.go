package filestore_util

import (
	errs "errors"
	"fmt"
	"io"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	bk "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	node "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
)

type ToFix struct {
	key  bk.Key
	good []bk.Key
}

func RepairPins(n *core.IpfsNode, fs *Datastore, wtr io.Writer, dryRun bool, skipRoot bool) error {
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
			if skipRoot {
				fmt.Fprintf(wtr, "  and removing recursive pin %s\n", to_fix.key)
			} else {
				fmt.Fprintf(wtr, "  and converting %s to a direct pin\n", to_fix.key)
			}
		} else {
			fmt.Fprintf(wtr, "Repairing recursive pin %s:\n", to_fix.key)
			for _, key := range to_fix.good {
				fmt.Fprintf(wtr, "  adding pin %s\n", key)
				pinning.RemovePinWithMode(key, pin.Direct)
				pinning.PinWithMode(key, pin.Recursive)
			}
			if skipRoot {
				fmt.Fprintf(wtr, "  removing recursive pin %s\n", to_fix.key)
				pinning.RemovePinWithMode(to_fix.key, pin.Recursive)
			} else {
				fmt.Fprintf(wtr, "  converting %s to a direct pin\n", to_fix.key)
				pinning.RemovePinWithMode(to_fix.key, pin.Recursive)
				pinning.PinWithMode(to_fix.key, pin.Direct)
			}
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
		return false, errs.New("Error when retrieving key")
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

func Unpinned(n *core.IpfsNode, fs *Datastore, wtr io.Writer) error {
	ls, err := ListWholeFile(fs)
	if err != nil {
		return err
	}
	unpinned := make(map[ds.Key]struct{})
	for res := range ls {
		if res.WholeFile() {
			unpinned[res.Key] = struct{}{}
		}
	}

	err = walkPins(n.Pinning, fs, n.Blockstore, func(key bk.Key, _ pin.PinMode) bool {
		dskey := key.DsKey()
		if _, ok := unpinned[dskey]; ok {
			delete(unpinned, dskey)
		}
		return true
	})
	if err != nil {
		return err
	}

	errors := false
	for key, _ := range unpinned {
		// We must retrieve the node and recomplete its hash
		// due to mangling of datastore keys
		bytes, err := fs.Get(key)
		if err != nil {
			errors = true
			continue
		}
		dagnode, err := node.DecodeProtobuf(bytes.([]byte))
		if err != nil {
			errors = true
			continue
		}
		k, err := dagnode.Key()
		if err != nil {
			errors = true
			continue
		}
		fmt.Fprintf(wtr, "%s\n", k)
	}
	if errors {
		return errs.New("Errors retrieving some keys, not all unpinned objects may be listed.")
	}
	return nil
}

//
// Walk the complete sets of pins and call mark on each run.  If mark
// returns true and the pin is due to a recursive pin, then
// recursively to check for indirect pins.
//
func walkPins(pinning pin.Pinner, fs *Datastore, bs b.Blockstore, mark func(bk.Key, pin.PinMode) bool) error {
	for _, k := range pinning.DirectKeys() {
		mark(k, pin.Direct)
	}
	var checkIndirect func(key bk.Key) error
	checkIndirect = func(key bk.Key) error {
		n, _, status := getNode(key.DsKey(), key, fs, bs)
		if AnError(status) {
			return errs.New("Error when retrieving key.")
		} else if n == nil {
			return nil
		}
		for _, link := range n.Links {
			if mark(bk.Key(link.Hash), pin.NotPinned) {
				checkIndirect(bk.Key(link.Hash))
			}
		}
		return nil
	}
	errors := false
	for _, k := range pinning.RecursiveKeys() {
		if mark(k, pin.Recursive) {
			err := checkIndirect(k)
			if err != nil {
				errors = true
			}
		}
	}
	if errors {
		return errs.New("Error when retrieving some keys.")
	}
	return nil
}
