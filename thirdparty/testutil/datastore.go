package testutil

import (
	ds2 "github.com/ipfs/go-ipfs/thirdparty/datastore2"
	"gx/ipfs/QmNgqJarToRiq2GBaPJhkmW4B5BxS5B74E1rkGvv2JoaTp/go-datastore"
	syncds "gx/ipfs/QmNgqJarToRiq2GBaPJhkmW4B5BxS5B74E1rkGvv2JoaTp/go-datastore/sync"
)

func ThreadSafeCloserMapDatastore() ds2.ThreadSafeDatastoreCloser {
	return ds2.CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore()))
}
