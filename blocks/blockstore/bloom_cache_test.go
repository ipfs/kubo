package blockstore

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs/blocks"

	context "context"
	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	dsq "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/query"
	syncds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/sync"
)

func testBloomCached(bs Blockstore, ctx context.Context) (*bloomcache, error) {
	if ctx == nil {
		ctx = context.TODO()
	}
	opts := DefaultCacheOpts()
	opts.HasARCCacheSize = 0
	bbs, err := CachedBlockstore(bs, ctx, opts)
	if err == nil {
		return bbs.(*bloomcache), nil
	} else {
		return nil, err
	}
}

func TestPutManyAddsToBloom(t *testing.T) {
	bs := NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))

	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	cachedbs, err := testBloomCached(bs, ctx)

	select {
	case <-cachedbs.rebuildChan:
	case <-ctx.Done():
		t.Fatalf("Timeout wating for rebuild: %d", cachedbs.bloom.ElementsAdded())
	}

	block1 := blocks.NewBlock([]byte("foo"))
	block2 := blocks.NewBlock([]byte("bar"))

	cachedbs.PutMany([]blocks.Block{block1})
	has, err := cachedbs.Has(block1.Cid())
	if err != nil {
		t.Fatal(err)
	}
	if has == false {
		t.Fatal("added block is reported missing")
	}

	has, err = cachedbs.Has(block2.Cid())
	if err != nil {
		t.Fatal(err)
	}
	if has == true {
		t.Fatal("not added block is reported to be in blockstore")
	}
}

func TestReturnsErrorWhenSizeNegative(t *testing.T) {
	bs := NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	_, err := bloomCached(bs, context.TODO(), -1, 1)
	if err == nil {
		t.Fail()
	}
}
func TestHasIsBloomCached(t *testing.T) {
	cd := &callbackDatastore{f: func() {}, ds: ds.NewMapDatastore()}
	bs := NewBlockstore(syncds.MutexWrap(cd))

	for i := 0; i < 1000; i++ {
		bs.Put(blocks.NewBlock([]byte(fmt.Sprintf("data: %d", i))))
	}
	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
	cachedbs, err := testBloomCached(bs, ctx)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-cachedbs.rebuildChan:
	case <-ctx.Done():
		t.Fatalf("Timeout wating for rebuild: %d", cachedbs.bloom.ElementsAdded())
	}

	cacheFails := 0
	cd.SetFunc(func() {
		cacheFails++
	})

	for i := 0; i < 1000; i++ {
		cachedbs.Has(blocks.NewBlock([]byte(fmt.Sprintf("data: %d", i+2000))).Cid())
	}

	if float64(cacheFails)/float64(1000) > float64(0.05) {
		t.Fatal("Bloom filter has cache miss rate of more than 5%")
	}

	cacheFails = 0
	block := blocks.NewBlock([]byte("newBlock"))

	cachedbs.PutMany([]blocks.Block{block})
	if cacheFails != 2 {
		t.Fatalf("expected two datastore hits: %d", cacheFails)
	}
	cachedbs.Put(block)
	if cacheFails != 3 {
		t.Fatalf("expected datastore hit: %d", cacheFails)
	}

	if has, err := cachedbs.Has(block.Cid()); !has || err != nil {
		t.Fatal("has gave wrong response")
	}

	bl, err := cachedbs.Get(block.Cid())
	if bl.String() != block.String() {
		t.Fatal("block data doesn't match")
	}

	if err != nil {
		t.Fatal("there should't be an error")
	}
}

type callbackDatastore struct {
	sync.Mutex
	f  func()
	ds ds.Datastore
}

func (c *callbackDatastore) SetFunc(f func()) {
	c.Lock()
	defer c.Unlock()
	c.f = f
}

func (c *callbackDatastore) CallF() {
	c.Lock()
	defer c.Unlock()
	c.f()
}

func (c *callbackDatastore) Put(key ds.Key, value interface{}) (err error) {
	c.CallF()
	return c.ds.Put(key, value)
}

func (c *callbackDatastore) Get(key ds.Key) (value interface{}, err error) {
	c.CallF()
	return c.ds.Get(key)
}

func (c *callbackDatastore) Has(key ds.Key) (exists bool, err error) {
	c.CallF()
	return c.ds.Has(key)
}

func (c *callbackDatastore) Delete(key ds.Key) (err error) {
	c.CallF()
	return c.ds.Delete(key)
}

func (c *callbackDatastore) Query(q dsq.Query) (dsq.Results, error) {
	c.CallF()
	return c.ds.Query(q)
}

func (c *callbackDatastore) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(c), nil
}
