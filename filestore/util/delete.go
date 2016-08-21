package filestore_util

import (
	errs "errors"
	"fmt"
	"io"

	. "github.com/ipfs/go-ipfs/filestore"

	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	//"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	k "github.com/ipfs/go-ipfs/blocks/key"
	//ds "github.com/ipfs/go-datastore"
	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	node "github.com/ipfs/go-ipfs/merkledag"
	//"github.com/ipfs/go-ipfs/pin"
)

type DeleteOpts struct {
	Direct     bool
	Continue   bool
}

type delInfo int

const (
	DirectlySpecified   delInfo = 1
	IndirectlySpecified delInfo = 2
)

func Delete(req cmds.Request, out io.Writer, node *core.IpfsNode, fs *Datastore, opts DeleteOpts, keyList ...k.Key) error {
	keys := make(map[k.Key]delInfo)
	for _, k := range keyList {
		keys[k] = DirectlySpecified
	}

	//
	// First check files
	//
	errors := false
	for _, k := range keyList {
		dagNode, dataObj, err := fsGetNode(k.DsKey(), fs)
		if err != nil {
			fmt.Fprintf(out, "%s: %s\n", k, err.Error())
			delete(keys, k)
			errors = true
			continue
		}
		if !opts.Direct && !dataObj.WholeFile() {
			fmt.Fprintf(out, "%s: part of another file, use --direct to delete\n", k)
			delete(keys, k)
			errors = true
			continue
		}
		if dagNode != nil && !opts.Direct {
			err = getChildren(out, dagNode, fs, node.Blockstore, keys)
			if err != nil {
				errors = true
			}
		}
	}
	if !opts.Continue && errors {
		return errs.New("Errors during precheck.")
	}

	//
	// Now check pins
	//

	// First get the set of pinned blocks
	keysKeys := make([]k.Key, 0, len(keys))
	for key, _ := range keys {
		keysKeys = append(keysKeys, key)
	}
	pinned, err := node.Pinning.CheckIfPinned(keysKeys...)
	if err != nil {
		return err
	}
	// Now check if removing any of the pinned blocks are stored
	// elsewhere if so, no problem and continue
	//for _, key := range pinned {
	//	
	//}
	stillPinned := pinned
	pinned = nil // save some space
	for _,inf := range stillPinned {
		if inf.Pinned() {
			fmt.Fprintf(out, "%s: %s\n", inf.Key, inf)
			errors = true
			delete(keys, inf.Key)
		}
	}
	if !opts.Continue && errors {
		return errs.New("Errors during pin check.")
	}
	stillPinned = nil // save some space

	//
	//
	//
	for key, _ := range keys {
		err := fs.DeleteDirect(key.DsKey())
		if err != nil {
			fmt.Fprintf(out, "%s: %s\n", key, err.Error())
		}
		fmt.Fprintf(out, "deleted %s\n", key)
	}

	if errors {
		return errs.New("Errors deleting some keys.")
	}
	return nil
}

func getChildren(out io.Writer, node *node.Node, fs *Datastore, bs b.Blockstore, keys map[k.Key]delInfo) error {
	errors := false
	for _, link := range node.Links {
		key := k.Key(link.Hash)
		if _, ok := keys[key]; ok {
			continue
		}
		n, _, status := getNode(key.DsKey(), key, fs, bs)
		if AnError(status) {
			fmt.Fprintf(out, "%s: error retrieving key", key)
			errors = true
		}
		keys[key] = IndirectlySpecified
		if n != nil {
			err := getChildren(out, n, fs, bs, keys)
			if err != nil {
				errors = true
			}
		}
	}
	if errors {
		return errs.New("Could net get all children.")
	}
	return nil
}
