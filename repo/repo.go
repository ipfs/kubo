package repo

import (
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	config "github.com/jbenet/go-ipfs/repo/config"
	util "github.com/jbenet/go-ipfs/util"
)

type Repo interface {
	Config() *config.Config
	SetConfig(*config.Config) error

	SetConfigKey(key string, value interface{}) error
	GetConfigKey(key string) (interface{}, error)

	Datastore() datastore.ThreadSafeDatastore
}

// IsInitialized returns true if the path is home to an initialized IPFS
// repository.
func IsInitialized(path string) bool {
	if !util.FileExists(path) {
		return false
	}
	// TODO add logging check
	// TODO add datastore check
	// TODO add config file check
	return true
}
