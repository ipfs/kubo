package repo

import (
	"io"

	datastore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	keystore "github.com/ipfs/go-ipfs/keystore"
	config "github.com/ipfs/go-ipfs/repo/config"
)

type Repo interface {
	Config() *config.Config
	SetConfig(*config.Config) error

	SetConfigKey(key string, value interface{}) error
	GetConfigKey(key string) (interface{}, error)

	Datastore() datastore.ThreadSafeDatastore

	Keystore() keystore.Keystore

	io.Closer
}
