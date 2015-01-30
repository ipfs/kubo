package blockservice

import (
	"bytes"
	"testing"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	offline "github.com/jbenet/go-ipfs/exchange/offline"
	blocks "github.com/jbenet/go-ipfs/struct/blocks"
	blockstore "github.com/jbenet/go-ipfs/struct/blocks/blockstore"
	blocksutil "github.com/jbenet/go-ipfs/struct/blocks/blocksutil"
	u "github.com/jbenet/go-ipfs/util"
)

func TestBlocks(t *testing.T) {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bs, err := New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Error("failed to construct block service", err)
		return
	}
	defer bs.Close()

	b := blocks.NewBlock([]byte("beep boop"))
	h := u.Hash([]byte("beep boop"))
	if !bytes.Equal(b.Multihash, h) {
		t.Error("Block Multihash and data multihash not equal")
	}

	if b.Key() != u.Key(h) {
		t.Error("Block key and data multihash key not equal")
	}

	k, err := bs.AddBlock(b)
	if err != nil {
		t.Error("failed to add block to BlockService", err)
		return
	}

	if k != b.Key() {
		t.Error("returned key is not equal to block key", err)
	}

	ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
	b2, err := bs.GetBlock(ctx, b.Key())
	if err != nil {
		t.Error("failed to retrieve block from BlockService", err)
		return
	}

	if b.Key() != b2.Key() {
		t.Error("Block keys not equal.")
	}

	if !bytes.Equal(b.Data, b2.Data) {
		t.Error("Block data is not equal.")
	}
}

func TestGetBlocksSequential(t *testing.T) {
	var servs = Mocks(t, 4)
	for _, s := range servs {
		defer s.Close()
	}
	bg := blocksutil.NewBlockGenerator()
	blks := bg.Blocks(50)

	var keys []u.Key
	for _, blk := range blks {
		keys = append(keys, blk.Key())
		servs[0].AddBlock(blk)
	}

	t.Log("one instance at a time, get blocks concurrently")

	for i := 1; i < len(servs); i++ {
		ctx, _ := context.WithTimeout(context.TODO(), time.Second*50)
		out := servs[i].GetBlocks(ctx, keys)
		gotten := make(map[u.Key]*blocks.Block)
		for blk := range out {
			if _, ok := gotten[blk.Key()]; ok {
				t.Fatal("Got duplicate block!")
			}
			gotten[blk.Key()] = blk
		}
		if len(gotten) != len(blks) {
			t.Fatalf("Didnt get enough blocks back: %d/%d", len(gotten), len(blks))
		}
	}
}
