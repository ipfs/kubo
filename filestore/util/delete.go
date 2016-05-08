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
	//ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	node "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
)

type DeleteOpts struct {
	Direct     bool
	Force      bool
	IgnorePins bool
}

func Delete(req cmds.Request, out io.Writer, node *core.IpfsNode, fs *Datastore, opts DeleteOpts, keyList ...k.Key) error {
	keys := make(map[k.Key]struct{})
	for _, k := range keyList {
		keys[k] = struct{}{}
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
	if !opts.Force && errors {
		return errs.New("Errors during precheck.")
	}

	//
	// Now check pins
	//
	pinned := make(map[k.Key]pin.PinMode)
	if !opts.IgnorePins {
		walkPins(node.Pinning, fs, node.Blockstore, func(key k.Key, mode pin.PinMode) bool {
			_, ok := keys[key]
			if !ok {
				// Hack to make sure mangled hashes are unpinned
				// (see issue #2601)
				_, ok = keys[k.KeyFromDsKey(key.DsKey())]
			}
			if ok {
				if mode == pin.NotPinned {
					// an indirect pin
					fmt.Fprintf(out, "%s: indirectly pinned\n", key)
					if !opts.Force {
						errors = true
					}
					return true
				} else {
					pinned[key] = mode
					return false
				}
			} else {
				return true
			}
		})
		if !opts.Force && errors {
			return errs.New("Errors during pin-check.")
		}
	}

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

	for key, mode := range pinned {
		stillExists, err := node.Blockstore.Has(key)
		if err != nil {
			fmt.Fprintf(out, "skipping pin %s: %s\n", err.Error())
			continue
		} else if stillExists {
			fmt.Fprintf(out, "skipping pin %s: object still exists outside filestore\n", key)
			continue
		}
		node.Pinning.RemovePinWithMode(key, mode)
		fmt.Fprintf(out, "unpinned %s\n", key)
	}
	if len(pinned) > 0 {
		err := node.Pinning.Flush()
		if err != nil {
			return err
		}
	}

	if errors {
		return errs.New("Errors deleting some keys.")
	}
	return nil
}

func getChildren(out io.Writer, node *node.Node, fs *Datastore, bs b.Blockstore, keys map[k.Key]struct{}) error {
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
		keys[key] = struct{}{}
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
