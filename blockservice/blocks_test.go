package blockservice

import (
	"bytes"
	"testing"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	blocks "github.com/jbenet/go-ipfs/blocks"
	blockstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	blocksutil "github.com/jbenet/go-ipfs/blocks/blocksutil"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
	tn "github.com/jbenet/go-ipfs/exchange/bitswap/testnet"
	offline "github.com/jbenet/go-ipfs/exchange/offline"
	"github.com/jbenet/go-ipfs/routing/mock"
	u "github.com/jbenet/go-ipfs/util"
)

func TestBlocks(t *testing.T) {
	d := ds.NewMapDatastore()
	tsds := dssync.MutexWrap(d)
	bs, err := New(blockstore.NewBlockstore(tsds), offline.Exchange())
	if err != nil {
		t.Error("failed to construct block service", err)
		return
	}

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
	net := tn.VirtualNetwork()
	rs := mock.VirtualRoutingServer()
	sg := bitswap.NewSessionGenerator(net, rs)
	bg := blocksutil.NewBlockGenerator()

	instances := sg.Instances(4)
	blks := bg.Blocks(50)
	// TODO: verify no duplicates

	var servs []*BlockService
	for _, i := range instances {
		bserv, err := New(i.Blockstore, i.Exchange)
		if err != nil {
			t.Fatal(err)
		}
		servs = append(servs, bserv)
	}

	var keys []u.Key
	for _, blk := range blks {
		keys = append(keys, blk.Key())
		servs[0].AddBlock(blk)
	}

	t.Log("one instance at a time, get blocks concurrently")

	for i := 1; i < len(instances); i++ {
		ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
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
