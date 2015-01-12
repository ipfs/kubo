package repo

import (
	config "github.com/jbenet/go-ipfs/repo/config"
	util "github.com/jbenet/go-ipfs/util"
)

type Interface interface {
	SetConfig(*config.Config) error
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
