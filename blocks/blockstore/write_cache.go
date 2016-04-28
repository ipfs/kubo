package blockstore

import (
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-ipfs/blocks"
	key "github.com/ipfs/go-ipfs/blocks/key"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

// WriteCached returns a blockstore that caches up to |size| unique writes (bs.Put).
func WriteCached(bs Blockstore, size int) (*writecache, error) {
	c, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &writecache{blockstore: bs, cache: c}, nil
}

type writecache struct {
	cache      *lru.Cache // pointer b/c Cache contains a Mutex as value (complicates copying)
	blockstore Blockstore
}

func (w *writecache) DeleteBlock(k key.Key) error {
	defer log.EventBegin(context.TODO(), "writecache.BlockRemoved", &k).Done()
	w.cache.Remove(k)
	return w.blockstore.DeleteBlock(k)
}

func (w *writecache) Has(k key.Key) (bool, error) {
	if _, ok := w.cache.Get(k); ok {
		return true, nil
	}
	return w.blockstore.Has(k)
}

func (w *writecache) Get(k key.Key) (blocks.Block, error) {
	return w.blockstore.Get(k)
}

func (w *writecache) Put(b blocks.Block) error {
	// Don't cache "advance" blocks
	if _, ok := b.(*blocks.BasicBlock); ok {
		k := b.Key()
		if _, ok := w.cache.Get(k); ok {
			return nil
		}
		defer log.EventBegin(context.TODO(), "writecache.BlockAdded", &k).Done()

		w.cache.Add(b.Key(), struct{}{})
	}
	return w.blockstore.Put(b)
}

func (w *writecache) PutMany(bs []blocks.Block) error {
	var good []blocks.Block
	for _, b := range bs {
		// Don't cache "advance" blocks
		if _, ok := b.(*blocks.BasicBlock); ok {
			if _, ok := w.cache.Get(b.Key()); !ok {
				good = append(good, b)
				k := b.Key()
				defer log.EventBegin(context.TODO(), "writecache.BlockAdded", &k).Done()
			}
		} else {
			good = append(good, b)
		}
	}
	return w.blockstore.PutMany(good)
}

func (w *writecache) AllKeysChan(ctx context.Context) (<-chan key.Key, error) {
	return w.blockstore.AllKeysChan(ctx)
}

func (w *writecache) GCLock() Unlocker {
	return w.blockstore.(GCBlockstore).GCLock()
}

func (w *writecache) PinLock() Unlocker {
	return w.blockstore.(GCBlockstore).PinLock()
}

func (w *writecache) GCRequested() bool {
	return w.blockstore.(GCBlockstore).GCRequested()
}
