package filestore_support

// A very simple multi-blockstore
// Put will only go to the first store

import (
	//"errors"
	"context"

	blocks "github.com/ipfs/go-ipfs/blocks"
	bls "github.com/ipfs/go-ipfs/blocks/blockstore"

	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	dsq "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/query"
)

func NewMultiBlockstore(stores ...bls.Blockstore) *multiblockstore {
	return &multiblockstore{
		stores: stores,
	}
}

type multiblockstore struct {
	stores []bls.Blockstore
}

func (bs *multiblockstore) DeleteBlock(key *cid.Cid) error {
	// FIXME: Delete from all stores
	return bs.stores[0].DeleteBlock(key)
}

func (bs *multiblockstore) Has(c *cid.Cid) (bool, error) {
	var firstErr error
	for _, b := range bs.stores {
		have, err := b.Has(c)
		if have && err == nil {
			return have, nil
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return false, firstErr
}

func (bs *multiblockstore) Get(c *cid.Cid) (blocks.Block, error) {
	var firstErr error
	for _, b := range bs.stores {
		blk, err := b.Get(c)
		if err == nil {
			return blk, nil
		}
		if firstErr == nil || firstErr == bls.ErrNotFound {
			firstErr = err
		}
	}
	return nil, firstErr
}

func (bs *multiblockstore) Put(blk blocks.Block) error {
	// First call Has() to make sure the block doesn't exist in any of
	// the sub-blockstores, otherwise we could end with data being
	// duplicated in two blockstores.
	exists, err := bs.Has(blk.Cid())
	if err == nil && exists {
		return nil // already stored
	}
	return bs.stores[0].Put(blk)
}

func (bs *multiblockstore) PutMany(blks []blocks.Block) error {
	stilladd := make([]blocks.Block, 0, len(blks))
	// First call Has() to make sure the block doesn't exist in any of
	// the sub-blockstores, otherwise we could end with data being
	// duplicated in two blockstores.
	for _, blk := range blks {
		exists, err := bs.Has(blk.Cid())
		if err == nil && exists {
			continue // already stored
		}
		stilladd = append(stilladd, blk)
	}
	if len(stilladd) == 0 {
		return nil
	}
	return bs.stores[0].PutMany(stilladd)
}

func (bs *multiblockstore) AllKeysChan(ctx context.Context) (<-chan *cid.Cid, error) {
	//return bs.stores[0].Blocks.AllKeysChan(ctx)
	//return nil, errors.New("Unimplemented")
	in := make([]<-chan *cid.Cid, 0, len(bs.stores))
	for _, b := range bs.stores {
		ch, err := b.AllKeysChan(ctx)
		if err != nil {
			return nil, err
		}
		in = append(in, ch)
	}
	out := make(chan *cid.Cid, dsq.KeysOnlyBufSize)
	go func() {
		defer close(out)
		for _, in0 := range in {
			for key := range in0 {
				out <- key
			}
		}
	}()
	return out, nil
}
