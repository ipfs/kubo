package blockstore

import (
	"github.com/ipfs/go-ipfs/blocks"
	key "github.com/ipfs/go-ipfs/blocks/key"
	lru "gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
	bloom "gx/ipfs/QmWQ2SJisXwcCLsUXLwYCKSfyExXjFRW2WbBH5sqCUnwX5/bbloom"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	ds "gx/ipfs/QmfQzVugPq1w5shWRcLWSeiHF4a2meBX7yVD8Vw7GWJM9o/go-datastore"
)

// BloomCached returns Blockstore that caches Has requests using Bloom filter
// Size is size of bloom filter in bytes
func BloomCached(bs Blockstore, bloomSize, lruSize int) (*bloomcache, error) {
	bl, err := bloom.New(float64(bloomSize), float64(7))
	if err != nil {
		return nil, err
	}
	arc, err := lru.NewARC(lruSize)
	if err != nil {
		return nil, err
	}
	bc := &bloomcache{blockstore: bs, bloom: bl, arc: arc}
	bc.Invalidate()
	go bc.Rebuild()

	return bc, nil
}

type bloomcache struct {
	bloom  *bloom.Bloom
	active bool

	arc *lru.ARCCache
	// This chan is only used for testing to wait for bloom to enable
	rebuildChan chan struct{}
	blockstore  Blockstore

	// Statistics
	hits   uint64
	misses uint64
}

func (b *bloomcache) Invalidate() {
	b.rebuildChan = make(chan struct{})
	b.active = false
}

func (b *bloomcache) BloomActive() bool {
	return b.active
}

func (b *bloomcache) Rebuild() {
	ctx := context.TODO()
	evt := log.EventBegin(ctx, "bloomcache.Rebuild")
	defer evt.Done()

	ch, err := b.blockstore.AllKeysChan(ctx)
	if err != nil {
		log.Errorf("AllKeysChan failed in bloomcache rebuild with: %v", err)
		return
	}
	for key := range ch {
		b.bloom.AddTS([]byte(key)) // Use binary key, the more compact the better
	}
	close(b.rebuildChan)
	b.active = true
}

func (b *bloomcache) DeleteBlock(k key.Key) error {
	if has, ok := b.hasCached(k); ok && !has {
		return ErrNotFound
	}

	b.arc.Remove(k) // Invalidate cache before deleting.
	err := b.blockstore.DeleteBlock(k)
	if err == nil {
		b.arc.Add(k, false)
	} else if err == ds.ErrNotFound || err == ErrNotFound {
		b.arc.Add(k, false)
		return ErrNotFound
	}
	return err
}

// if ok == false has is inconclusive
// if ok == true then has respons to question: is it contained
func (b *bloomcache) hasCached(k key.Key) (has bool, ok bool) {
	if k == "" {
		return true, true
	}
	if b.active {
		blr := b.bloom.HasTS([]byte(k))
		if blr == false { // not contained in bloom is only conclusive answer bloom gives
			return blr, true
		}
	}
	h, ok := b.arc.Get(k)
	if ok {
		return h.(bool), ok
	} else {
		return false, ok
	}
}

func (b *bloomcache) Has(k key.Key) (bool, error) {
	if has, ok := b.hasCached(k); ok {
		return has, nil
	}

	res, err := b.blockstore.Has(k)
	if err == nil {
		b.arc.Add(k, res)
	}
	return res, err
}

func (b *bloomcache) Get(k key.Key) (blocks.Block, error) {
	if has, ok := b.hasCached(k); ok && !has {
		return nil, ErrNotFound
	}

	bl, err := b.blockstore.Get(k)
	if bl == nil && err == ErrNotFound {
		b.arc.Add(k, false)
	} else if bl != nil {
		b.arc.Add(k, true)
	}
	return bl, err
}

func (b *bloomcache) Put(bl blocks.Block) error {
	if has, ok := b.hasCached(bl.Key()); ok && has {
		return nil
	}

	err := b.blockstore.Put(bl)
	if err == nil {
		b.bloom.AddTS([]byte(bl.Key()))
		b.arc.Add(bl.Key(), true)
	}
	return err
}

func (b *bloomcache) PutMany(bs []blocks.Block) error {
	var good []blocks.Block
	for _, block := range bs {
		if has, ok := b.hasCached(block.Key()); !ok || (ok && !has) {
			good = append(good, block)
		}
	}
	err := b.blockstore.PutMany(bs)
	if err == nil {
		for _, block := range bs {
			b.bloom.AddTS([]byte(block.Key()))
		}
	}
	return err
}

func (b *bloomcache) AllKeysChan(ctx context.Context) (<-chan key.Key, error) {
	return b.blockstore.AllKeysChan(ctx)
}

func (b *bloomcache) GCLock() Unlocker {
	return b.blockstore.(GCBlockstore).GCLock()
}

func (b *bloomcache) PinLock() Unlocker {
	return b.blockstore.(GCBlockstore).PinLock()
}

func (b *bloomcache) GCRequested() bool {
	return b.blockstore.(GCBlockstore).GCRequested()
}
