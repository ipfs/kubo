package blockservice

import (
	"testing"

	"github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	butil "github.com/ipfs/go-ipfs/blocks/blocksutil"
	offline "github.com/ipfs/go-ipfs/exchange/offline"

	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	dssync "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/sync"
)

func TestWriteThroughWorks(t *testing.T) {
	bstore := &PutCountingBlockstore{
		blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore())),
		0,
	}
	bstore2 := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	exch := offline.Exchange(bstore2)
	bserv := NewWriteThrough(bstore, exch)
	bgen := butil.NewBlockGenerator()

	block := bgen.Next()

	t.Logf("PutCounter: %d", bstore.PutCounter)
	bserv.AddBlock(block)
	if bstore.PutCounter != 1 {
		t.Fatalf("expected just one Put call, have: %d", bstore.PutCounter)
	}

	bserv.AddBlock(block)
	if bstore.PutCounter != 2 {
		t.Fatalf("Put should have called again, should be 2 is: %d", bstore.PutCounter)
	}
}

var _ blockstore.Blockstore = (*PutCountingBlockstore)(nil)

type PutCountingBlockstore struct {
	blockstore.Blockstore
	PutCounter int
}

func (bs *PutCountingBlockstore) Put(block blocks.Block) error {
	bs.PutCounter++
	return bs.Blockstore.Put(block)
}
