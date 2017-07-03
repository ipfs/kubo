package testutil

import (
	ds2 "github.com/ipfs/go-ipfs/thirdparty/datastore2"
	"gx/ipfs/QmSiN66ybp5udnQnvhb6euiWiiQWdGvwMhAWa95cC1DTCV/go-datastore"
	syncds "gx/ipfs/QmSiN66ybp5udnQnvhb6euiWiiQWdGvwMhAWa95cC1DTCV/go-datastore/sync"
)

func ThreadSafeCloserMapDatastore() ds2.ThreadSafeDatastoreCloser {
	return ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore()))
}
