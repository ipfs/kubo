package repo

import (
	"errors"
	"io"

	config "github.com/ipfs/go-ipfs/repo/config"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
)

var (
	ErrApiNotRunning = errors.New("api not running")
)

type Repo interface {
	Config() (*config.Config, error)
	SetConfig(*config.Config) error

	SetConfigKey(key string, value interface{}) error
	GetConfigKey(key string) (interface{}, error)

	Datastore() Datastore
	GetStorageUsage() (uint64, error)

	SubDatastore(key string) Datastore

	// SetAPIAddr sets the API address in the repo.
	SetAPIAddr(addr string) error

	io.Closer
}

// Datastore is the interface required from a datastore to be
// acceptable to FSRepo.
type Datastore interface {
	ds.Batching // should be threadsafe, just be careful
	io.Closer
}
