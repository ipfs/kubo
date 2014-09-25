package core

import (
	"fmt"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	lds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go/leveldb"
	config "github.com/jbenet/go-ipfs/config"
)

func makeDatastore(cfg config.Datastore) (ds.Datastore, error) {
	if len(cfg.Type) == 0 {
		return nil, fmt.Errorf("config datastore.type required")
	}

	switch cfg.Type {
	case "leveldb":
		return makeLevelDBDatastore(cfg)
	case "memory":
		return ds.NewMapDatastore(), nil
	}

	return nil, fmt.Errorf("Unknown datastore type: %s", cfg.Type)
}

func makeLevelDBDatastore(cfg config.Datastore) (ds.Datastore, error) {
	path, err := cfg.GetPath()
	if err != nil {
		return nil, err
	}

	if len(path) == 0 {
		return nil, fmt.Errorf("config datastore.path required for leveldb")
	}

	return lds.NewDatastore(path, nil)
}
