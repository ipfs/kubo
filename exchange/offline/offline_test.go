package offline

import (
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ds_sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	blocks "github.com/jbenet/go-ipfs/struct/blocks"
	"github.com/jbenet/go-ipfs/struct/blocks/blockstore"
	"github.com/jbenet/go-ipfs/struct/blocks/blocksutil"
	u "github.com/jbenet/go-ipfs/util"
)

func TestBlockReturnsErr(t *testing.T) {
	off := Exchange(bstore())
	_, err := off.GetBlock(context.Background(), u.Key("foo"))
	if err != nil {
		return // as desired
	}
	t.Fail()
}

func TestHasBlockReturnsNil(t *testing.T) {
	store := bstore()
	ex := Exchange(store)
	block := blocks.NewBlock([]byte("data"))

	err := ex.HasBlock(context.Background(), block)
	if err != nil {
		t.Fail()
	}

	if _, err := store.Get(block.Key()); err != nil {
		t.Fatal(err)
	}
}

func TestGetBlocks(t *testing.T) {
	store := bstore()
	ex := Exchange(store)
	g := blocksutil.NewBlockGenerator()

	expected := g.Blocks(2)

	for _, b := range expected {
		if err := ex.HasBlock(context.Background(), b); err != nil {
			t.Fail()
		}
	}

	request := func() []u.Key {
		var ks []u.Key

		for _, b := range expected {
			ks = append(ks, b.Key())
		}
		return ks
	}()

	received, err := ex.GetBlocks(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for _ = range received {
		count++
	}
	if len(expected) != count {
		t.Fail()
	}
}

func bstore() blockstore.Blockstore {
	return blockstore.NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
}
