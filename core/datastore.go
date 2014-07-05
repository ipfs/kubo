package core

import (
  config "github.com/jbenet/go-ipfs/config"
  ds "github.com/jbenet/datastore.go"
  "fmt"
  lds "github.com/jbenet/datastore.go/leveldb"
)

func makeDatastore(cfg *config.Datastore) (ds.Datastore, error) {
  if cfg == nil || len(cfg.Type) == 0 {
    return nil, fmt.Errorf("config datastore.type required")
  }

  switch cfg.Type {
  case "leveldb": return makeLevelDBDatastore(cfg)
  case "memory": return ds.NewMapDatastore(), nil
  }

  return nil, fmt.Errorf("Unknown datastore type: %s", cfg.Type)
}

func makeLevelDBDatastore(cfg *config.Datastore) (ds.Datastore, error) {
  if len(cfg.Path) == 0 {
    return nil, fmt.Errorf("config datastore.path required for leveldb")
  }

  return lds.NewDatastore(cfg.Path, nil)
}
