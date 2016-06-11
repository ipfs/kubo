package blockstore

import (
	"testing"

	"github.com/ipfs/go-ipfs/blocks"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	dsq "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/query"
	syncds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/sync"
)

func TestReturnsErrorWhenSizeNegative(t *testing.T) {
	bs := NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	_, err := WriteCached(bs, -1)
	if err != nil {
		return
	}
	t.Fail()
}

func TestRemoveCacheEntryOnDelete(t *testing.T) {
	b := blocks.NewBlock([]byte("foo"))
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	bs := NewBlockstore(syncds.MutexWrap(cd))
	cachedbs, err := WriteCached(bs, 1)
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
	cachedbs, err := WriteCached(bs, 1)
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
