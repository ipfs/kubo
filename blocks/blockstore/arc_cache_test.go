package blockstore

import (
	"github.com/ipfs/go-ipfs/blocks"
	"testing"

	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	syncds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/sync"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func testArcCached(bs GCBlockstore, ctx context.Context) (*arccache, error) {
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

func TestRemoveCacheEntryOnDelete(t *testing.T) {
	b := blocks.NewBlock([]byte("foo"))
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	bs := NewBlockstore(syncds.MutexWrap(cd))
	cachedbs, err := testArcCached(bs, nil)
	if err != nil {
		t.Fatal(err)
	}
	cachedbs.Put(b)

	cd.Lock()
	writeHitTheDatastore := false
	cd.Unlock()

	cd.SetFunc(func() {
		writeHitTheDatastore = true
	})

	cachedbs.DeleteBlock(b.Key())
	cachedbs.Put(b)
	if !writeHitTheDatastore {
		t.Fail()
	}
}

func TestElideDuplicateWrite(t *testing.T) {
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	bs := NewBlockstore(syncds.MutexWrap(cd))
	cachedbs, err := testArcCached(bs, nil)
	if err != nil {
		t.Fatal(err)
	}

	b1 := blocks.NewBlock([]byte("foo"))

	cachedbs.Put(b1)
	cd.SetFunc(func() {
		t.Fatal("write hit the datastore")
	})
	cachedbs.Put(b1)
}
