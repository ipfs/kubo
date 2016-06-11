package datastore2

import (
	"io"

	"gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
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
