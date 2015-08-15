package mdutils

import (
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	bsrv "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	dag "github.com/ipfs/go-ipfs/merkledag"
)

func Mock() dag.DAGService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bserv := bsrv.New(bstore, offline.Exchange(bstore))
	return dag.NewDAGService(bserv)
}
