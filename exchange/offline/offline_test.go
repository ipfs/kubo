package offline

import (
	"context"
	"testing"

	"github.com/ipfs/go-ipfs/blocks/blockstore"
	"github.com/ipfs/go-ipfs/blocks/blocksutil"
	blocks "gx/ipfs/QmbJUay5h1HtzhJb5QQk2t26yCnJksHynvhcqp18utBPqG/go-block-format"

	cid "gx/ipfs/QmNw61A6sJoXMeP37mJRtQZdNhj5e3FdjoTN3v4FyE96Gk/go-cid"
	ds "gx/ipfs/QmSiN66ybp5udnQnvhb6euiWiiQWdGvwMhAWa95cC1DTCV/go-datastore"
	ds_sync "gx/ipfs/QmSiN66ybp5udnQnvhb6euiWiiQWdGvwMhAWa95cC1DTCV/go-datastore/sync"
	u "gx/ipfs/QmWbjfz3u6HkAdPh34dgPchGbQjob6LXLhAeCGii2TX69n/go-ipfs-util"
)

func TestBlockReturnsErr(t *testing.T) {
	off := Exchange(bstore())
	c := cid.NewCidV0(u.Hash([]byte("foo")))
	_, err := off.GetBlock(context.Background(), c)
	if err != nil {
		return // as desired
	}
	t.Fail()
}

func TestHasBlockReturnsNil(t *testing.T) {
	store := bstore()
	ex := Exchange(store)
	block := blocks.NewBlock([]byte("data"))

	err := ex.HasBlock(block)
	if err != nil {
		t.Fail()
	}

	if _, err := store.Get(block.Cid()); err != nil {
		t.Fatal(err)
	}
}

func TestGetBlocks(t *testing.T) {
	store := bstore()
	ex := Exchange(store)
	g := blocksutil.NewBlockGenerator()

	expected := g.Blocks(2)

	for _, b := range expected {
		if err := ex.HasBlock(b); err != nil {
			t.Fail()
		}
	}

	request := func() []*cid.Cid {
		var ks []*cid.Cid

		for _, b := range expected {
			ks = append(ks, b.Cid())
		}
		return ks
	}()

	received, err := ex.GetBlocks(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for range received {
		count++
	}
	if len(expected) != count {
		t.Fail()
	}
}

func bstore() blockstore.Blockstore {
	return blockstore.NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
}
