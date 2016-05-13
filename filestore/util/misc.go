package filestore_util

import (
	"io"
	"fmt"

	. "github.com/ipfs/go-ipfs/filestore"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
)

func RmDups(wtr io.Writer, fs *Datastore, bs b.Blockstore) error {
	ls, err := ListKeys(fs)
	if err != nil {return err}
	for res := range ls {
		key := k.KeyFromDsKey(res.Key)
		// This is a quick and dirty hack.  Right now the
		// filestore ignores normal delete requests so
		// deleting a block from the blockstore will delete it
		// form the normal datastore but form the filestore
		err := bs.DeleteBlock(key)
		if err == nil {
			fmt.Fprintf(wtr, "deleted duplicate %s\n", key)
		}
	}
	return nil
}
