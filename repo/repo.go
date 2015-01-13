package repo

import (
	config "github.com/jbenet/go-ipfs/repo/config"
	util "github.com/jbenet/go-ipfs/util"
)

type Interface interface {
	Config() *config.Config
	SetConfig(*config.Config) error

	SetConfigKey(key string, value interface{}) error
	GetConfigKey(key string) (interface{}, error)
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
