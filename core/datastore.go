package core

import (
	"fmt"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	fsds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/fs"
	ktds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/keytransform"
	lds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/leveldb"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	config "github.com/jbenet/go-ipfs/config"
	u "github.com/jbenet/go-ipfs/util"
)

func makeDatastore(cfg config.Datastore) (ds.ThreadSafeDatastore, error) {
	if len(cfg.Type) == 0 {
		return nil, fmt.Errorf("config datastore.type required")
	}

	switch cfg.Type {
	case "leveldb":
		return makeLevelDBDatastore(cfg)

	case "memory":
		return syncds.MutexWrap(ds.NewMapDatastore()), nil

	case "fs":
		log.Warning("using fs.Datastore at .datastore for testing.")
		d, err := fsds.NewDatastore(".datastore") // for testing!!
		if err != nil {
			return nil, err
		}
		ktd := ktds.WrapDatastore(d, u.DsKeyB58Encode)
		return syncds.MutexWrap(ktd), nil
	}

	return nil, fmt.Errorf("Unknown datastore type: %s", cfg.Type)
}

func makeLevelDBDatastore(cfg config.Datastore) (ds.ThreadSafeDatastore, error) {
	if len(cfg.Path) == 0 {
		return nil, fmt.Errorf("config datastore.path required for leveldb")
	}

	return lds.NewDatastore(cfg.Path, nil)
}
