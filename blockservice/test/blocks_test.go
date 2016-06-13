package bstest

import (
	"bytes"
	"testing"
	"time"

	blocks "github.com/ipfs/go-ipfs/blocks"
	blockstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	blocksutil "github.com/ipfs/go-ipfs/blocks/blocksutil"
	key "github.com/ipfs/go-ipfs/blocks/key"
	. "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	dssync "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/sync"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func TestBlocks(t *testing.T) {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bs := New(bstore, offline.Exchange(bstore))
	defer bs.Close()

	_, err := bs.GetBlock(context.Background(), key.Key(""))
	if err != ErrNotFound {
		t.Error("Empty String Key should error", err)
	}

	b := blocks.NewBlock([]byte("beep boop"))
	h := u.Hash([]byte("beep boop"))
	if !bytes.Equal(b.Multihash(), h) {
		t.Error("Block Multihash and data multihash not equal")
	}

	if b.Key() != key.Key(h) {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	b2, err := bs.GetBlock(ctx, b.Key())
	if err != nil {
		t.Error("failed to retrieve block from BlockService", err)
		return
	}

	if b.Key() != b2.Key() {
		t.Error("Block keys not equal.")
	}

	if !bytes.Equal(b.Data(), b2.Data()) {
		t.Error("Block data is not equal.")
	}
}

func TestGetBlocksSequential(t *testing.T) {
	var servs = Mocks(4)
	for _, s := range servs {
		defer s.Close()
	}
	bg := blocksutil.NewBlockGenerator()
	blks := bg.Blocks(50)

	var keys []key.Key
	for _, blk := range blks {
		keys = append(keys, blk.Key())
		servs[0].AddBlock(blk)
	}

	t.Log("one instance at a time, get blocks concurrently")

	for i := 1; i < len(servs); i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
		defer cancel()
		out := servs[i].GetBlocks(ctx, keys)
		gotten := make(map[key.Key]blocks.Block)
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
