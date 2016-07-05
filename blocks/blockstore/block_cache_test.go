package blockstore

import (
	"testing"

	"github.com/ipfs/go-ipfs/blocks"

	ds "gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore"
	syncds "gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore/sync"
)

func TestBlocksAreCachedOnPut(t *testing.T) {
	b := blocks.NewBlock([]byte("foo"))
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	bs := NewBlockstore(syncds.MutexWrap(cd))
	cachedbs, err := blockCached(bs, 16)
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

	cachedbs.Get(b.Key())
	if writeHitTheDatastore {
		t.Fail()
	}

}

func TestBlocksAreCachedOnGet(t *testing.T) {
	b := blocks.NewBlock([]byte("foo"))
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	bs := NewBlockstore(syncds.MutexWrap(cd))
	cachedbs, err := blockCached(bs, 16)
	if err != nil {
		t.Fatal(err)
	}
	bs.Put(b)

	cd.Lock()
	hits := 0
	cd.Unlock()

	cd.SetFunc(func() {
		hits += 1
	})

	cachedbs.Get(b.Key())
	cachedbs.Get(b.Key())
	cachedbs.Get(b.Key())

	if hits != 1 {
		t.Fail()
	}

}
