package merkledag

import (
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/jbenet/go-ipfs/exchange/offline"
	bsrv "github.com/jbenet/go-ipfs/struct/blocks/blockservice"
	"github.com/jbenet/go-ipfs/struct/blocks/blockstore"
)

func Mock(t testing.TB) DAGService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bserv, err := bsrv.New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Fatal(err)
	}
	return NewDAGService(bserv)
}
