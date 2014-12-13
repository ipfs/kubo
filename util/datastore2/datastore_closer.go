package datastore2

import (
	"io"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
)

type ThreadSafeDatastoreCloser interface {
	datastore.ThreadSafeDatastore
	io.Closer
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
