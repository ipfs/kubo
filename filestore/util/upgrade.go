package filestore_util

import (
	"fmt"
	"io"

	. "github.com/ipfs/go-ipfs/filestore"

	//b "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

func Upgrade(wtr io.Writer, fs *Datastore) error {
	ls, err := ListAll(fs)
	if err != nil {
		return err
	}
	cnt := 0
	for r := range ls {
		dsKey := r.Key
		key, err := k.KeyFromDsKey(r.Key)
		if err != nil {
			key = k.Key(r.Key.String()[1:])
			dsKey = key.DsKey()
		}
		if len(dsKey.String()) != 56 {
			data, err := fs.GetData(r.Key, r.DataObj, VerifyNever, true);
			if err != nil {
				fmt.Fprintf(wtr, "error: could not fix invalid key %s: %s\n",
					key.String(), err.Error())
			} else {
				key = k.Key(u.Hash(data))
				dsKey = key.DsKey()
			}
				
		}
		err = fs.PutDirect(dsKey, r.DataObj)
		if err != nil {
			return err
		}
		if !dsKey.Equal(r.Key) {
			err = fs.Delete(r.Key)
			if err != nil {
				return err
			}
		}
		cnt++
	}
	fmt.Fprintf(wtr, "Upgraded %d entries.\n", cnt)
	return nil
}
