package blockstore

import (
	"testing"

	"github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/blocks/key"

	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	syncds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/sync"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var exampleBlock = blocks.NewBlock([]byte("foo"))

func testArcCached(bs Blockstore, ctx context.Context) (*arccache, error) {
	if ctx == nil {
		ctx = context.TODO()
	}
	opts := DefaultCacheOpts()
	opts.HasBloomFilterSize = 0
	opts.HasBloomFilterHashes = 0
	bbs, err := CachedBlockstore(bs, ctx, opts)
	if err == nil {
		return bbs.(*arccache), nil
	} else {
		return nil, err
	}
}

func createStores(t *testing.T) (*arccache, *blockstore, *callbackDatastore) {
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	bs := NewBlockstore(syncds.MutexWrap(cd))
	arc, err := testArcCached(bs, nil)
	if err != nil {
		t.Fatal(err)
	}
	return arc, bs, cd
}

func trap(message string, cd *callbackDatastore, t *testing.T) {
	cd.SetFunc(func() {
		t.Fatal(message)
	})
}
func untrap(cd *callbackDatastore) {
	cd.SetFunc(func() {})
}

func TestRemoveCacheEntryOnDelete(t *testing.T) {
	arc, _, cd := createStores(t)

	arc.Put(exampleBlock)

	cd.Lock()
	writeHitTheDatastore := false
	cd.Unlock()

	cd.SetFunc(func() {
		writeHitTheDatastore = true
	})

	arc.DeleteBlock(exampleBlock.Key())
	arc.Put(exampleBlock)
	if !writeHitTheDatastore {
		t.Fail()
	}
}

func TestElideDuplicateWrite(t *testing.T) {
	arc, _, cd := createStores(t)

	arc.Put(exampleBlock)
	trap("write hit datastore", cd, t)
	arc.Put(exampleBlock)
}

func TestHasRequestTriggersCache(t *testing.T) {
	arc, _, cd := createStores(t)

	arc.Has(exampleBlock.Key())
	trap("has hit datastore", cd, t)
	if has, err := arc.Has(exampleBlock.Key()); has || err != nil {
		t.Fatal("has was true but there is no such block")
	}

	untrap(cd)
	err := arc.Put(exampleBlock)
	if err != nil {
		t.Fatal(err)
	}

	trap("has hit datastore", cd, t)

	if has, err := arc.Has(exampleBlock.Key()); !has || err != nil {
		t.Fatal("has returned invalid result")
	}
}

func TestGetFillsCache(t *testing.T) {
	arc, _, cd := createStores(t)

	if bl, err := arc.Get(exampleBlock.Key()); bl != nil || err == nil {
		t.Fatal("block was found or there was no error")
	}

	trap("has hit datastore", cd, t)

	if has, err := arc.Has(exampleBlock.Key()); has || err != nil {
		t.Fatal("has was true but there is no such block")
	}

	untrap(cd)

	if err := arc.Put(exampleBlock); err != nil {
		t.Fatal(err)
	}

	trap("has hit datastore", cd, t)

	if has, err := arc.Has(exampleBlock.Key()); !has || err != nil {
		t.Fatal("has returned invalid result")
	}
}

func TestGetAndDeleteFalseShortCircuit(t *testing.T) {
	arc, _, cd := createStores(t)

	arc.Has(exampleBlock.Key())

	trap("get hit datastore", cd, t)

	if bl, err := arc.Get(exampleBlock.Key()); bl != nil || err != ErrNotFound {
		t.Fatal("get returned invalid result")
	}

	if arc.DeleteBlock(exampleBlock.Key()) != ErrNotFound {
		t.Fatal("expected ErrNotFound error")
	}
}

func TestArcCreationFailure(t *testing.T) {
	if arc, err := arcCached(nil, -1); arc != nil || err == nil {
		t.Fatal("expected error and no cache")
	}
}

func TestInvalidKey(t *testing.T) {
	arc, _, _ := createStores(t)

	bl, err := arc.Get(key.Key(""))

	if bl != nil {
		t.Fatal("blocks should be nil")
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHasAfterSucessfulGetIsCached(t *testing.T) {
	arc, bs, cd := createStores(t)

	bs.Put(exampleBlock)

	arc.Get(exampleBlock.Key())

	trap("has hit datastore", cd, t)
	arc.Has(exampleBlock.Key())
}

func TestPutManyCaches(t *testing.T) {
	arc, _, cd := createStores(t)
	arc.PutMany([]blocks.Block{exampleBlock})

	trap("has hit datastore", cd, t)
	arc.Has(exampleBlock.Key())
}
