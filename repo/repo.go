package repo

import (
	"errors"
	"io"

	datastore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	config "github.com/ipfs/go-ipfs/repo/config"
)

var (
	ErrApiNotRunning = errors.New("api not running")
)

type Repo interface {
	Config() (*config.Config, error)
	SetConfig(*config.Config) error

	SetConfigKey(key string, value interface{}) error
	GetConfigKey(key string) (interface{}, error)

	Datastore() datastore.ThreadSafeDatastore

	// SetAPIAddr sets the API address in the repo.
	SetAPIAddr(addr string) error

	io.Closer
}
