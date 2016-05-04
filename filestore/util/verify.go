package filestore_util

import (
	"io"

	. "github.com/ipfs/go-ipfs/filestore"
)

func VerifyBlocks(wtr io.Writer, fs *Datastore) error {
	ch, _ := List(fs, false)
	for res := range ch {
		if !res.NoBlockData() {
			continue
		}
		res.Status = verify(fs, res.Key, res.DataObj)
		wtr.Write([]byte(res.Format()))
	}
	return nil
}

