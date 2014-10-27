package testutil

import (
	"testing"
	crand "crypto/rand"

	"github.com/jbenet/go-ipfs/peer"

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

func RandPeer() peer.Peer {
	id := make(peer.ID, 16)
	crand.Read(id)
	return peer.WithID(id)
}