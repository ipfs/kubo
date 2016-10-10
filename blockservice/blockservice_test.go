package blockservice

import (
	"testing"

	"github.com/ipfs/go-ipfs/blocks/blockstore"
	butil "github.com/ipfs/go-ipfs/blocks/blocksutil"
	offline "github.com/ipfs/go-ipfs/exchange/offline"

	cid "gx/ipfs/QmakyCk6Vnn16WEKjbkxieZmM2YLTzkFWizbmGowoYPjro/go-cid"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	dssync "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/sync"
)

func TestWriteThroughWorks(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := HasFailingBlockstore{
		blockstore.NewBlockstore(dstore),
		t,
		true,
	}
	exch := offline.Exchange(bstore)
	bserv := NewWriteThrough(bstore, exch)
	bgen := butil.NewBlockGenerator()

	bserv.AddBlock(bgen.Next())
}

var _ blockstore.GCBlockstore = (*HasFailingBlockstore)(nil)

type HasFailingBlockstore struct {
	blockstore.GCBlockstore
	t    *testing.T
	Fail bool
}

func (bs HasFailingBlockstore) Has(k *cid.Cid) (bool, error) {
	if bs.Fail {
		bs.t.Fatal("Has shouldn't be called")
		return false, nil
	}
	return bs.GCBlockstore.Has(k)

}
