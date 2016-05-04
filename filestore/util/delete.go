package filestore_util

import (
	"fmt"
	"io"

	. "github.com/ipfs/go-ipfs/filestore"

	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	k "github.com/ipfs/go-ipfs/blocks/key"
	
)

func Delete(req cmds.Request, out io.Writer, node *core.IpfsNode, fs *Datastore, key k.Key) error {
	err := fs.DeleteDirect(key.DsKey())
	if err != nil {
		return err
	}
	stillExists, err := node.Blockstore.Has(key)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Deleted %s\n", key)
	if stillExists {
		return nil
	}
	_, pinned1, err := node.Pinning.IsPinnedWithType(key, "recursive")
	if err != nil {
		return err
	}
	_, pinned2, err := node.Pinning.IsPinnedWithType(key, "direct")
	if err != nil {
		return err
	}
	if pinned1 || pinned2 {
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		err = node.Pinning.Unpin(ctx, key, true)
		if err != nil {
			return err
		}
		err := node.Pinning.Flush()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Unpinned %s\n", key)
	}
	return nil
}
