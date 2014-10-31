package testutil

import (
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	bsrv "github.com/jbenet/go-ipfs/blockservice"
	dag "github.com/jbenet/go-ipfs/merkledag"
)

func GetDAGServ(t testing.TB) dag.DAGService {
	dstore := ds.NewMapDatastore()
	bserv, err := bsrv.NewBlockService(dstore, nil)
	if err != nil {
		t.Fatal(err)
	}
	return dag.NewDAGService(bserv)
}
