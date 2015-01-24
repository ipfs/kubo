package blockstore

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/hashicorp/golang-lru"

	"github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

// WriteCached returns a blockstore that caches up to |size| unique writes (bs.Put).
func WriteCached(bs Blockstore, size int) (Blockstore, error) {
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

func (w *writecache) DeleteBlock(k u.Key) error {
	w.cache.Remove(k)
	return w.blockstore.DeleteBlock(k)
}

func (w *writecache) Has(k u.Key) (bool, error) {
	if _, ok := w.cache.Get(k); ok {
		return true, nil
	}
	return w.blockstore.Has(k)
}

func (w *writecache) Get(k u.Key) (*blocks.Block, error) {
	return w.blockstore.Get(k)
}

func (w *writecache) Put(b *blocks.Block) error {
	if _, ok := w.cache.Get(b.Key()); ok {
		return nil
	}
	w.cache.Add(b.Key(), struct{}{})
	return w.blockstore.Put(b)
}

func (w *writecache) AllKeys(ctx context.Context) ([]u.Key, error) {
	return w.blockstore.AllKeysRange(ctx, 0, 0)
}

func (w *writecache) AllKeysChan(ctx context.Context) (<-chan u.Key, error) {
	return w.blockstore.AllKeysRangeChan(ctx, 0, 0)
}

func (w *writecache) AllKeysRange(ctx context.Context, offset int, limit int) ([]u.Key, error) {
	return w.blockstore.AllKeysRange(ctx, offset, limit)
}

func (w *writecache) AllKeysRangeChan(ctx context.Context, offset int, limit int) (<-chan u.Key, error) {
	return w.blockstore.AllKeysRangeChan(ctx, offset, limit)
}
