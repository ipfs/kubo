package blockstore

import (
	"context"
	"sync/atomic"
	"time"

	"gx/ipfs/QmVA4mafxbfH5aEvNz8fyoxC6J1xhAtw88B4GerPznSZBg/go-block-format"

	"gx/ipfs/QmRg1gKTHzc3CZXSKzem8aR4E3TubFhbgXwfVuWnSK5CC5/go-metrics-interface"
	cid "gx/ipfs/QmTprEaAA2A9bst5XH7exuyi5KzNMK3SEDNN8rBDnKWcUS/go-cid"
	bloom "gx/ipfs/QmXqKGu7QzfRzFC4yd5aL9sThYx22vY163VGwmxfp5qGHk/bbloom"
)

// bloomCached returns a Blockstore that caches Has requests using a Bloom
// filter. bloomSize is size of bloom filter in bytes. hashCount specifies the
// number of hashing functions in the bloom filter (usually known as k).
func bloomCached(ctx context.Context, bs Blockstore, bloomSize, hashCount int) (*bloomcache, error) {
	bl, err := bloom.New(float64(bloomSize), float64(hashCount))
	if err != nil {
		return nil, err
	}
	bc := &bloomcache{Blockstore: bs, bloom: bl}
	bc.hits = metrics.NewCtx(ctx, "bloom.hits_total",
		"Number of cache hits in bloom cache").Counter()
	bc.total = metrics.NewCtx(ctx, "bloom_total",
		"Total number of requests to bloom cache").Counter()

	bc.Invalidate()
	go bc.Rebuild(ctx)
	if metrics.Active() {
		go func() {
			fill := metrics.NewCtx(ctx, "bloom_fill_ratio",
				"Ratio of bloom filter fullnes, (updated once a minute)").Gauge()

			<-bc.rebuildChan
			t := time.NewTicker(1 * time.Minute)
			for {
				select {
				case <-ctx.Done():
					t.Stop()
					return
				case <-t.C:
					fill.Set(bc.bloom.FillRatio())
				}
			}
		}()
	}
	return bc, nil
}

type bloomcache struct {
	Blockstore

	bloom  *bloom.Bloom
	active int32

	// This chan is only used for testing to wait for bloom to enable
	rebuildChan chan struct{}

	// Statistics
	hits  metrics.Counter
	total metrics.Counter
}

func (b *bloomcache) Invalidate() {
	b.rebuildChan = make(chan struct{})
	atomic.StoreInt32(&b.active, 0)
}

func (b *bloomcache) BloomActive() bool {
	return atomic.LoadInt32(&b.active) != 0
}

func (b *bloomcache) Rebuild(ctx context.Context) {
	evt := log.EventBegin(ctx, "bloomcache.Rebuild")
	defer evt.Done()

	ch, err := b.Blockstore.AllKeysChan(ctx)
	if err != nil {
		log.Errorf("AllKeysChan failed in bloomcache rebuild with: %v", err)
		return
	}
	finish := false
	for !finish {
		select {
		case key, ok := <-ch:
			if ok {
				b.bloom.AddTS(key.Bytes()) // Use binary key, the more compact the better
			} else {
				finish = true
			}
		case <-ctx.Done():
			log.Warning("Cache rebuild closed by context finishing.")
			return
		}
	}
	close(b.rebuildChan)
	atomic.StoreInt32(&b.active, 1)
}

func (b *bloomcache) DeleteBlock(k *cid.Cid) error {
	if has, ok := b.hasCached(k); ok && !has {
		return ErrNotFound
	}

	return b.Blockstore.DeleteBlock(k)
}

// if ok == false has is inconclusive
// if ok == true then has respons to question: is it contained
func (b *bloomcache) hasCached(k *cid.Cid) (has bool, ok bool) {
	b.total.Inc()
	if k == nil {
		log.Error("nil cid in bloom cache")
		// Return cache invalid so call to blockstore
		// in case of invalid key is forwarded deeper
		return false, false
	}
	if b.BloomActive() {
		blr := b.bloom.HasTS(k.Bytes())
		if !blr { // not contained in bloom is only conclusive answer bloom gives
			b.hits.Inc()
			return false, true
		}
	}
	return false, false
}

func (b *bloomcache) Has(k *cid.Cid) (bool, error) {
	if has, ok := b.hasCached(k); ok {
		return has, nil
	}

	return b.Blockstore.Has(k)
}

func (b *bloomcache) Get(k *cid.Cid) (blocks.Block, error) {
	if has, ok := b.hasCached(k); ok && !has {
		return nil, ErrNotFound
	}

	return b.Blockstore.Get(k)
}

func (b *bloomcache) Put(bl blocks.Block) error {
	// See comment in PutMany
	err := b.Blockstore.Put(bl)
	if err == nil {
		b.bloom.AddTS(bl.Cid().Bytes())
	}
	return err
}

func (b *bloomcache) PutMany(bs []blocks.Block) error {
	// bloom cache gives only conclusive resulty if key is not contained
	// to reduce number of puts we need conclusive information if block is contained
	// this means that PutMany can't be improved with bloom cache so we just
	// just do a passthrough.
	err := b.Blockstore.PutMany(bs)
	if err != nil {
		return err
	}
	for _, bl := range bs {
		b.bloom.AddTS(bl.Cid().Bytes())
	}
	return nil
}
