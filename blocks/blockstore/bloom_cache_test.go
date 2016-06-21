package blockstore

import (
	"fmt"
	"github.com/ipfs/go-ipfs/blocks"
	ds "gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore"
	dsq "gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore/query"
	syncds "gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore/sync"
	"testing"
	"time"
)

func TestReturnsErrorWhenSizeNegative(t *testing.T) {
	bs := NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	_, err := BloomCached(bs, 100, -1)
	if err == nil {
		t.Fail()
	}
	_, err = BloomCached(bs, -1, 100)
	if err == nil {
		t.Fail()
	}
}

func TestRemoveCacheEntryOnDelete(t *testing.T) {
	b := blocks.NewBlock([]byte("foo"))
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	bs := NewBlockstore(syncds.MutexWrap(cd))
	cachedbs, err := BloomCached(bs, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	cachedbs.Put(b)

	writeHitTheDatastore := false
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
	cachedbs, err := BloomCached(bs, 1, 1)
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
func TestHasIsBloomCached(t *testing.T) {
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	bs := NewBlockstore(syncds.MutexWrap(cd))

	for i := 0; i < 1000; i++ {
		bs.Put(blocks.NewBlock([]byte(fmt.Sprintf("data: %d", i))))
	}
	cachedbs, err := BloomCached(bs, 256*1024, 128)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-cachedbs.rebuildChan:
	case <-time.After(1 * time.Second):
		t.Fatalf("Timeout wating for rebuild: %d", cachedbs.bloom.ElementsAdded())
	}

	cacheFails := 0
	cd.SetFunc(func() {
		cacheFails++
	})

	for i := 0; i < 1000; i++ {
		cachedbs.Has(blocks.NewBlock([]byte(fmt.Sprintf("data: %d", i+2000))).Key())
	}

	if float64(cacheFails)/float64(1000) > float64(0.05) {
		t.Fatal("Bloom filter has cache miss rate of more than 5%")
	}
}

type callbackDatastore struct {
	f  func()
	ds ds.Datastore
}

func (c *callbackDatastore) SetFunc(f func()) { c.f = f }

func (c *callbackDatastore) Put(key ds.Key, value interface{}) (err error) {
	c.f()
	return c.ds.Put(key, value)
}

func (c *callbackDatastore) Get(key ds.Key) (value interface{}, err error) {
	c.f()
	return c.ds.Get(key)
}

func (c *callbackDatastore) Has(key ds.Key) (exists bool, err error) {
	c.f()
	return c.ds.Has(key)
}

func (c *callbackDatastore) Delete(key ds.Key) (err error) {
	c.f()
	return c.ds.Delete(key)
}

func (c *callbackDatastore) Query(q dsq.Query) (dsq.Results, error) {
	c.f()
	return c.ds.Query(q)
}

func (c *callbackDatastore) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(c), nil
}
