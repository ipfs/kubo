package mdutils

import (
	"testing"

	bsrv "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks/blockservice"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks/blockstore"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/ipfs/go-ipfs/exchange/offline"
	dag "github.com/ipfs/go-ipfs/merkledag"
)

func Mock(t testing.TB) dag.DAGService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bserv, err := bsrv.New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Fatal(err)
	}
	return dag.NewDAGService(bserv)
}
