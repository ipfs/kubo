package bitswap

import (
	"context"
	"fmt"
	"testing"
	"time"

	blocks "github.com/ipfs/go-ipfs/blocks"
	blocksutil "github.com/ipfs/go-ipfs/blocks/blocksutil"

	cid "gx/ipfs/Qma4RJSuh7mMeJQYCqMbKzekn6EwBo7HEs5AQYjVRMQATB/go-cid"
)

func TestBasicSessions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vnet := getVirtualNetwork()
	sesgen := NewTestSessionGenerator(vnet)
	defer sesgen.Close()
	bgen := blocksutil.NewBlockGenerator()

	block := bgen.Next()
	inst := sesgen.Instances(2)

	a := inst[0]
	b := inst[1]

	if err := b.Blockstore().Put(block); err != nil {
		t.Fatal(err)
	}

	sesa := a.Exchange.NewSession(ctx)

	blkout, err := sesa.GetBlock(ctx, block.Cid())
	if err != nil {
		t.Fatal(err)
	}

	if !blkout.Cid().Equals(block.Cid()) {
		t.Fatal("got wrong block")
	}
}

func assertBlockLists(got, exp []blocks.Block) error {
	if len(got) != len(exp) {
		return fmt.Errorf("got wrong number of blocks, %d != %d", len(got), len(exp))
	}

	h := cid.NewSet()
	for _, b := range got {
		h.Add(b.Cid())
	}
	for _, b := range exp {
		if !h.Has(b.Cid()) {
			return fmt.Errorf("didnt have: %s", b.Cid())
		}
	}
	return nil
}

func TestSessionBetweenPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vnet := getVirtualNetwork()
	sesgen := NewTestSessionGenerator(vnet)
	defer sesgen.Close()
	bgen := blocksutil.NewBlockGenerator()

	inst := sesgen.Instances(10)

	blks := bgen.Blocks(101)
	if err := inst[0].Blockstore().PutMany(blks); err != nil {
		t.Fatal(err)
	}

	var cids []*cid.Cid
	for _, blk := range blks {
		cids = append(cids, blk.Cid())
	}

	ses := inst[1].Exchange.NewSession(ctx)
	if _, err := ses.GetBlock(ctx, cids[0]); err != nil {
		t.Fatal(err)
	}
	blks = blks[1:]
	cids = cids[1:]

	for i := 0; i < 10; i++ {
		ch, err := ses.GetBlocks(ctx, cids[i*10:(i+1)*10])
		if err != nil {
			t.Fatal(err)
		}

		var got []blocks.Block
		for b := range ch {
			got = append(got, b)
		}
		if err := assertBlockLists(got, blks[i*10:(i+1)*10]); err != nil {
			t.Fatal(err)
		}
	}
	for _, is := range inst[2:] {
		if is.Exchange.messagesRecvd > 2 {
			t.Fatal("uninvolved nodes should only receive two messages", is.Exchange.messagesRecvd)
		}
	}
}

func TestSessionSplitFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vnet := getVirtualNetwork()
	sesgen := NewTestSessionGenerator(vnet)
	defer sesgen.Close()
	bgen := blocksutil.NewBlockGenerator()

	inst := sesgen.Instances(11)

	blks := bgen.Blocks(100)
	for i := 0; i < 10; i++ {
		if err := inst[i].Blockstore().PutMany(blks[i*10 : (i+1)*10]); err != nil {
			t.Fatal(err)
		}
	}

	var cids []*cid.Cid
	for _, blk := range blks {
		cids = append(cids, blk.Cid())
	}

	ses := inst[10].Exchange.NewSession(ctx)
	ses.baseTickDelay = time.Millisecond * 10

	for i := 0; i < 10; i++ {
		ch, err := ses.GetBlocks(ctx, cids[i*10:(i+1)*10])
		if err != nil {
			t.Fatal(err)
		}

		var got []blocks.Block
		for b := range ch {
			got = append(got, b)
		}
		if err := assertBlockLists(got, blks[i*10:(i+1)*10]); err != nil {
			t.Fatal(err)
		}
	}
}
