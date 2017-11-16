package blockstore

import (
	"context"

	"gx/ipfs/QmSn9Td7xgxm9EV7iEjTckpUWmWApggzPxu7eFGWkkpwin/go-block-format"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	"gx/ipfs/QmRg1gKTHzc3CZXSKzem8aR4E3TubFhbgXwfVuWnSK5CC5/go-metrics-interface"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	lru "gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
)

// arccache wraps a BlockStore with an Adaptive Replacement Cache (ARC) for
// block Cids. This provides block access-time improvements, allowing
// to short-cut many searches without query-ing the underlying datastore.
type arccache struct {
	arc        *lru.ARCCache
	blockstore Blockstore

	hits  metrics.Counter
	total metrics.Counter
}

func newARCCachedBS(ctx context.Context, bs Blockstore, lruSize int) (*arccache, error) {
	arc, err := lru.NewARC(lruSize)
	if err != nil {
		return nil, err
	}
	c := &arccache{arc: arc, blockstore: bs}
	c.hits = metrics.NewCtx(ctx, "arc.hits_total", "Number of ARC cache hits").Counter()
	c.total = metrics.NewCtx(ctx, "arc_total", "Total number of ARC cache requests").Counter()

	return c, nil
}

func (b *arccache) DeleteBlock(k mh.Multihash) error {
	if has, ok := b.hasCached(k); ok && !has {
		return ErrNotFound
	}

	b.arc.Remove(string(k)) // Invalidate cache before deleting.
	err := b.blockstore.DeleteBlock(k)
	switch err {
	case nil, ds.ErrNotFound, ErrNotFound:
		b.addCache(k, false)
		return err
	default:
		return err
	}
}

// if ok == false has is inconclusive
// if ok == true then has respons to question: is it contained
func (b *arccache) hasCached(k mh.Multihash) (has bool, ok bool) {
	b.total.Inc()
	if k == nil {
		log.Error("nil cid in arccache")
		// Return cache invalid so the call to blockstore happens
		// in case of invalid key and correct error is created.
		return false, false
	}

	h, ok := b.arc.Get(string(k))
	if ok {
		b.hits.Inc()
		return h.(bool), true
	}
	return false, false
}

func (b *arccache) Has(k mh.Multihash) (bool, error) {
	if has, ok := b.hasCached(k); ok {
		return has, nil
	}

	res, err := b.blockstore.Has(k)
	if err == nil {
		b.addCache(k, res)
	}
	return res, err
}

func (b *arccache) Get(k *cid.Cid) (blocks.Block, error) {
	if k == nil {
		log.Error("nil cid in arc cache")
		return nil, ErrNotFound
	}

	if has, ok := b.hasCached(k.Hash()); ok && !has {
		return nil, ErrNotFound
	}

	bl, err := b.blockstore.Get(k)
	if bl == nil && err == ErrNotFound {
		b.addCache(k.Hash(), false)
	} else if bl != nil {
		b.addCache(k.Hash(), true)
	}
	return bl, err
}

func (b *arccache) Put(bl blocks.Block) error {
	if has, ok := b.hasCached(bl.Cid().Hash()); ok && has {
		return nil
	}

	err := b.blockstore.Put(bl)
	if err == nil {
		b.addCache(bl.Cid().Hash(), true)
	}
	return err
}

func (b *arccache) PutMany(bs []blocks.Block) error {
	var good []blocks.Block
	for _, block := range bs {
		// call put on block if result is inconclusive or we are sure that
		// the block isn't in storage
		if has, ok := b.hasCached(block.Cid().Hash()); !ok || (ok && !has) {
			good = append(good, block)
		}
	}
	err := b.blockstore.PutMany(good)
	if err != nil {
		return err
	}
	for _, block := range good {
		b.addCache(block.Cid().Hash(), true)
	}
	return nil
}

func (b *arccache) HashOnRead(enabled bool) {
	b.blockstore.HashOnRead(enabled)
}

func (b *arccache) addCache(k mh.Multihash, has bool) {
	b.arc.Add(string(k), has)
}

func (b *arccache) AllKeysChan(ctx context.Context) (<-chan mh.Multihash, error) {
	return b.blockstore.AllKeysChan(ctx)
}

func (b *arccache) GCLock() Unlocker {
	return b.blockstore.(GCBlockstore).GCLock()
}

func (b *arccache) PinLock() Unlocker {
	return b.blockstore.(GCBlockstore).PinLock()
}

func (b *arccache) GCRequested() bool {
	return b.blockstore.(GCBlockstore).GCRequested()
}
