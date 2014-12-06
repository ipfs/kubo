package blockstore

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/jbenet/go-ipfs/blocks"
	"testing"
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

func (c *callbackDatastore) KeyList() ([]ds.Key, error) {
	c.f()
	return c.ds.KeyList()
}
