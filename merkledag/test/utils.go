package mdutils

import (
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/jbenet/go-ipfs/blocks/blockstore"
	bsrv "github.com/jbenet/go-ipfs/blockservice"
	"github.com/jbenet/go-ipfs/exchange/offline"
	dag "github.com/jbenet/go-ipfs/merkledag"
)

func Mock(t testing.TB) dag.DAGService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bserv, err := bsrv.New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Fatal(err)
	}
	return dag.NewDAGService(bserv)
}
