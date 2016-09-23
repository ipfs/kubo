package repo

import (
	"errors"
	"io"

	config "github.com/ipfs/go-ipfs/repo/config"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
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

	// DirectMount provides direct access to a datastore mounted
	// under prefix in order to perform low-level operations.  The
	// datastore returned is guaranteed not be a proxy (such as a
	// go-datastore/measure) normal operations should go through
	// Datastore()
	DirectMount(prefix string) ds.Datastore
	Mounts() []string

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
