package filestore_util

import (
	"fmt"
	"io"

	. "github.com/ipfs/go-ipfs/filestore"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
)

func RmDups(wtr io.Writer, fs *Datastore, bs b.Blockstore) error {
	ls, err := ListKeys(fs)
	if err != nil {
		return err
	}
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

func Upgrade(wtr io.Writer, fs *Datastore) error {
	ls, err := ListAll(fs)
	if err != nil {
		return err
	}
	cnt := 0
	for res := range ls {
		err := fs.PutDirect(res.Key, res.DataObj)
		if err != nil {
			return err
		}
		cnt++
	}
	fmt.Fprintf(wtr, "Upgraded %d entries.\n", cnt)
	return nil
}
