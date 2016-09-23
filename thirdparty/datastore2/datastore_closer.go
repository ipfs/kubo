package datastore2

import (
	"io"

	"gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
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
