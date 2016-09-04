package datastore2

import (
	"io"

	"gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	syncds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/sync"
)

type ThreadSafeDatastoreCloser interface {
	datastore.ThreadSafeDatastore
	io.Closer

	Batch() (datastore.Batch, error)
}

func CloserWrap(ds datastore.ThreadSafeDatastore) ThreadSafeDatastoreCloser {
	return &datastoreCloserWrapper{ds}
}

type datastoreCloserWrapper struct {
	datastore.ThreadSafeDatastore
}

func (w *datastoreCloserWrapper) Close() error {
	return nil // no-op
}

func (w *datastoreCloserWrapper) Batch() (datastore.Batch, error) {
	bds, ok := w.ThreadSafeDatastore.(datastore.Batching)
	if !ok {
		return nil, datastore.ErrBatchUnsupported
	}

	return bds.Batch()
}

// TODO(geoah) Move to ./testutils or somewhere else
func ThreadSafeCloserMapDatastore() ThreadSafeDatastoreCloser {
	return CloserWrap(syncds.MutexWrap(datastore.NewMapDatastore()))
}
