package filestore_util

import (
	"fmt"
	"io"

	. "github.com/ipfs/go-ipfs/filestore"

	//b "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
)

func Upgrade(wtr io.Writer, fs *Datastore) error {
	iter := fs.NewIterator()
	cnt := 0
	for iter.Next() {
		origKey := iter.Key()
		dsKey := origKey
		key, err := k.KeyFromDsKey(origKey)
		if err != nil {
			key = k.Key(origKey.String()[1:])
			dsKey = key.DsKey()
		}
		bytes, val, err := iter.Value()
		if err != nil {
			return err
		}
		if len(dsKey.String()) != 56 {
			data, err := GetData(nil, origKey, bytes, val, VerifyNever)
			if err != nil {
				fmt.Fprintf(wtr, "error: could not fix invalid key %s: %s\n",
					key.String(), err.Error())
			} else {
				key = k.Key(u.Hash(data))
				dsKey = key.DsKey()
			}

		}
		_, err = fs.Update(dsKey.Bytes(), bytes, val)
		if err != nil {
			return err
		}
		if !dsKey.Equal(origKey) {
			err = fs.Delete(origKey)
			if err != nil {
				return err
			}
		}
		cnt++
	}
	fmt.Fprintf(wtr, "Upgraded %d entries.\n", cnt)
	return nil
}
