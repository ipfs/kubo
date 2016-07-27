package mdutils

import (
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	bsrv "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ds "gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore"
	dssync "gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore/sync"
)

func Mock() dag.DAGService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bserv := bsrv.New(bstore, offline.Exchange(bstore))
	return dag.NewDAGService(bserv)
}
