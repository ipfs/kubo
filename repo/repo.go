package repo

import (
	"io"

	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	config "github.com/jbenet/go-ipfs/repo/config"
)

type Repo interface {
	Config() *config.Config
	SetConfig(*config.Config) error

	SetConfigKey(key string, value interface{}) error
	GetConfigKey(key string) (interface{}, error)

	Datastore() datastore.ThreadSafeDatastore

	io.Closer
}
