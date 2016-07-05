package blockstore

import (
	"github.com/ipfs/go-ipfs/blocks"

	key "github.com/ipfs/go-ipfs/blocks/key"
	lru "gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

// blockCached caches blocks on Get and Put calls
// it is desigend to be behind HasBloom/ARC cache
func blockCached(bs Blockstore, arcsize int) (*blockcache, error) {
	arc, err := lru.NewARC(arcsize)
	if err != nil {
		return nil, err
	}
	bc := &blockcache{blockstore: bs, arc: arc}
	return bc, nil
}

type blockcache struct {
	arc        *lru.ARCCache
	blockstore Blockstore
}

func (b *blockcache) DeleteBlock(k key.Key) error {
	b.arc.Remove(k)
	return b.blockstore.DeleteBlock(k)
}

func (b *blockcache) Has(k key.Key) (bool, error) {
	// We are not checking cache here.
	// The ARC cache of bloomcache is orders of magnitude bigger
	// than this cache, which means that if bloomcache
	// didn't catch it we won't either
	return b.blockstore.Has(k)
}

func (b *blockcache) Get(k key.Key) (blocks.Block, error) {
	if block, ok := b.arc.Get(k); ok {
		// cache hit, yey !!
		// we just didn't read 256KiB from disk
		// (it might have been less but still)
		return block.(blocks.Block), nil
	}
	// cache miss but we will be able to add the result

	bl, err := b.blockstore.Get(k)
	if bl != nil && err == nil {
		// we are checking err for nil just to be extra sure
		// adding something bad into cache can be really bad
		// as cache is not storing errors
		b.arc.Add(k, bl)
	}
	return bl, err
}

func (b *blockcache) Put(bl blocks.Block) error {
	// Same as with Has, we are not checking the cache here
	// before calling Put, if bloomcache didn't catch it
	// we won't either

	err := b.blockstore.Put(bl)
	if err == nil {
		b.arc.Add(bl.Key(), bl)
	}
	return err
}

func (b *blockcache) PutMany(bs []blocks.Block) error {
	// we rely on bloomcache to only pass blocks that
	// are really being added, so same conditions as above

	err := b.blockstore.PutMany(bs)
	if err == nil {
		for _, block := range bs {
			b.arc.Add(block.Key(), block)
		}
	}
	return err
}

func (b *blockcache) AllKeysChan(ctx context.Context) (<-chan key.Key, error) {
	return b.blockstore.AllKeysChan(ctx)
}

func (b *blockcache) GCLock() Unlocker {
	return b.blockstore.(GCBlockstore).GCLock()
}

func (b *blockcache) PinLock() Unlocker {
	return b.blockstore.(GCBlockstore).PinLock()
}

func (b *blockcache) GCRequested() bool {
	return b.blockstore.(GCBlockstore).GCRequested()
}
