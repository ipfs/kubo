package mdutils

import (
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	bsrv "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ds "gx/ipfs/QmSiN66ybp5udnQnvhb6euiWiiQWdGvwMhAWa95cC1DTCV/go-datastore"
	dssync "gx/ipfs/QmSiN66ybp5udnQnvhb6euiWiiQWdGvwMhAWa95cC1DTCV/go-datastore/sync"
)

func Mock() dag.DAGService {
	return dag.NewDAGService(Bserv())
}

func Bserv() bsrv.BlockService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	return bsrv.New(bstore, offline.Exchange(bstore))
}
