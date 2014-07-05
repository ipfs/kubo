package core

import (
  "testing"
  config "github.com/jbenet/go-ipfs/config"
)

func TestDatastores(t *testing.T) {

  good := []*config.Config {
    &config.Config{ Datastore: &config.Datastore{Type: "memory"} },
    &config.Config{ Datastore: &config.Datastore{Type: "leveldb", Path: ".testdb"} },
  }

  bad := []*config.Config {
    &config.Config{ Datastore: &config.Datastore{} },
    &config.Config{ Datastore: &config.Datastore{Type: "badtype"} },
    &config.Config{ },
    nil,
  }

  for i, c := range(good) {
    n, err := NewIpfsNode(c)
    if n == nil || err != nil {
      t.Error("Should have constructed.", i, err)
    }
  }

  for i, c := range(bad) {
    n, err := NewIpfsNode(c)
    if n != nil || err == nil {
      t.Error("Should have failed to construct.", i)
    }
  }
}
