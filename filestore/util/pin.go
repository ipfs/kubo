package filestore_util

import (
	"errors"
	"fmt"
	"io"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	bk "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	node "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
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

func Repin(ctx0 context.Context, n *core.IpfsNode, fs *Datastore, wtr io.Writer) error {
	ls, err := List(fs, false)
	if err != nil {
		return err
	}
	unpinned := make(map[ds.Key]struct{})
	for res := range ls {
		if res.WholeFile() {
			unpinned[res.Key] = struct{}{}
		}
	}
	pinning := n.Pinning
	bs := n.Blockstore
	for _, k := range n.Pinning.DirectKeys() {
		if _, ok := unpinned[k.DsKey()]; ok {
			delete(unpinned, k.DsKey())
		}
	}
	var checkIndirect func(key bk.Key) error
	checkIndirect = func(key bk.Key) error {
		n, _, status := getNode(key.DsKey(), key, fs, bs)
		if AnError(status) {
			return errors.New("Error when retrieving key")
		} else if n == nil {
			return nil
		}
		for _, link := range n.Links {
			if _, ok := unpinned[ds.NewKey(string(link.Hash))]; ok {
				delete(unpinned, ds.NewKey(string(link.Hash)))
			}
			checkIndirect(bk.Key(link.Hash))
		}
		return nil
	}
	for _, k := range pinning.RecursiveKeys() {
		if _, ok := unpinned[k.DsKey()]; ok {
			delete(unpinned, k.DsKey())
		}
		err = checkIndirect(k)
		if err != nil {
			return err
		}
	}

	for key, _ := range unpinned {
		fmt.Fprintf(wtr, "Pinning %s\n", b58.Encode(key.Bytes()[1:]))
		bytes, err := fs.Get(key)
		if err != nil {
			return err
		}
		dagnode, err := node.DecodeProtobuf(bytes.([]byte))
		if err != nil {
			return err
		}
		ctx, cancel := context.WithCancel(ctx0)
		defer cancel()
		err = n.Pinning.Pin(ctx, dagnode, true)
		if err != nil {
			return err
		}

	}
	err = n.Pinning.Flush()
	if err != nil {
		return err
	}

	return nil
}
