package testutil

import (
	crand "crypto/rand"
	"testing"

	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/jbenet/go-ipfs/exchange/offline"
	"github.com/jbenet/go-ipfs/peer"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/jbenet/go-ipfs/blocks/blockstore"
	bsrv "github.com/jbenet/go-ipfs/blockservice"
	dag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

func GetDAGServ(t testing.TB) dag.DAGService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bserv, err := bsrv.New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Fatal(err)
	}
	return dag.NewDAGService(bserv)
}

func RandPeer() peer.Peer {
	id := make([]byte, 16)
	crand.Read(id)
	mhid := u.Hash(id)
	return peer.WithID(peer.ID(mhid))
}
