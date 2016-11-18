package blockstore

// A very simple multi-blockstore that analogous to a unionfs Put and
// DeleteBlock only go to the first blockstore all others are
// considered readonly.

import (
	//"errors"
	"context"

	blocks "github.com/ipfs/go-ipfs/blocks"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	dsq "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/query"
)

type LocateInfo struct {
	Prefix string
	Error  error
}

type MultiBlockstore interface {
	Blockstore
	GCLocker
	FirstMount() Blockstore
	Mounts() []string
	Mount(prefix string) Blockstore
	Locate(*cid.Cid) []LocateInfo
}

type Mount struct {
	Prefix string
	Blocks Blockstore
}

func NewMultiBlockstore(mounts ...Mount) *multiblockstore {
	return &multiblockstore{
		mounts: mounts,
	}
}

type multiblockstore struct {
	mounts []Mount
	gclocker
}

func (bs *multiblockstore) FirstMount() Blockstore {
	return bs.mounts[0].Blocks
}

func (bs *multiblockstore) Mounts() []string {
	mounts := make([]string, 0, len(bs.mounts))
	for _, mnt := range bs.mounts {
		mounts = append(mounts, mnt.Prefix)
	}
	return mounts
}

func (bs *multiblockstore) Mount(prefix string) Blockstore {
	for _, m := range bs.mounts {
		if m.Prefix == prefix {
			return m.Blocks
		}
	}
	return nil
}

func (bs *multiblockstore) DeleteBlock(key *cid.Cid) error {
	return bs.mounts[0].Blocks.DeleteBlock(key)
}

func (bs *multiblockstore) Has(c *cid.Cid) (bool, error) {
	var firstErr error
	for _, m := range bs.mounts {
		have, err := m.Blocks.Has(c)
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
	for _, m := range bs.mounts {
		blk, err := m.Blocks.Get(c)
		if err == nil {
			return blk, nil
		}
		if firstErr == nil || firstErr == ErrNotFound {
			firstErr = err
		}
	}
	return nil, firstErr
}

func (bs *multiblockstore) Locate(c *cid.Cid) []LocateInfo {
	res := make([]LocateInfo, 0, len(bs.mounts))
	for _, m := range bs.mounts {
		_, err := m.Blocks.Get(c)
		res = append(res, LocateInfo{m.Prefix, err})
	}
	return res
}

func (bs *multiblockstore) Put(blk blocks.Block) error {
	// First call Has() to make sure the block doesn't exist in any of
	// the sub-blockstores, otherwise we could end with data being
	// duplicated in two blockstores.
	exists, err := bs.Has(blk.Cid())
	if err == nil && exists {
		return nil // already stored
	}
	return bs.mounts[0].Blocks.Put(blk)
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
	return bs.mounts[0].Blocks.PutMany(stilladd)
}

func (bs *multiblockstore) AllKeysChan(ctx context.Context) (<-chan *cid.Cid, error) {
	//return bs.mounts[0].Blocks.AllKeysChan(ctx)
	//return nil, errors.New("Unimplemented")
	in := make([]<-chan *cid.Cid, 0, len(bs.mounts))
	for _, m := range bs.mounts {
		ch, err := m.Blocks.AllKeysChan(ctx)
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
