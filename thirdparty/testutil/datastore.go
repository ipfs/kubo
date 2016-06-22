package testutil

import (
	ds2 "github.com/ipfs/go-ipfs/thirdparty/datastore2"
	"gx/ipfs/QmbCg24DeRKaRDLHbzzSVj7xndmWCPanBLkAM7Lx2nbrFs/go-datastore"
	syncds "gx/ipfs/QmbCg24DeRKaRDLHbzzSVj7xndmWCPanBLkAM7Lx2nbrFs/go-datastore/sync"
)

func ThreadSafeCloserMapDatastore() ds2.ThreadSafeDatastoreCloser {
	return ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore()))
}
