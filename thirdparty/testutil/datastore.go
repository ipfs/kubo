package testutil

import (
	ds2 "github.com/ipfs/go-ipfs/thirdparty/datastore2"
	"gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore"
	syncds "gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore/sync"
)

func ThreadSafeCloserMapDatastore() ds2.ThreadSafeDatastoreCloser {
	return ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore()))
}
