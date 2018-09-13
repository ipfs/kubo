package cidv0v1

import (
	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	cidutil "gx/ipfs/QmdPF1WZQHFNfLdwhaShiR3e4KvFviAM58TrxVxPMhukic/go-cidutil"
	bs "gx/ipfs/QmdriVJgKx4JADRgh3cYPXqXmsa1A45SvFki1nDWHhQNtC/go-ipfs-blockstore"
)

type blockstore struct {
	bs.Blockstore
}

func NewBlockstore(b bs.Blockstore) bs.Blockstore {
	return &blockstore{b}
}

func (b *blockstore) Has(c cid.Cid) (bool, error) {
	have, err := b.Blockstore.Has(c)
	if have || err != nil {
		return have, err
	}
	c1 := cidutil.TryOtherCidVersion(c)
	if !c1.Defined() {
		return false, nil
	}
	return b.Blockstore.Has(c1)
}

func (b *blockstore) Get(c cid.Cid) (blocks.Block, error) {
	block, err := b.Blockstore.Get(c)
	if err == nil {
		return block, nil
	}
	if err != bs.ErrNotFound {
		return nil, err
	}
	c1 := cidutil.TryOtherCidVersion(c)
	if !c1.Defined() {
		return nil, bs.ErrNotFound
	}
	block, err = b.Blockstore.Get(c1)
	if err != nil {
		return nil, err
	}
	// modify block so it has the original CID
	block, err = blocks.NewBlockWithCid(block.RawData(), c)
	if err != nil {
		return nil, err
	}
	// insert the block with the original CID to avoid problems
	// with pinning
	err = b.Blockstore.Put(block)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (b *blockstore) GetSize(c cid.Cid) (int, error) {
	size, err := b.Blockstore.GetSize(c)
	if err == nil {
		return size, nil
	}
	if err != bs.ErrNotFound {
		return -1, err
	}
	c1 := cidutil.TryOtherCidVersion(c)
	if !c1.Defined() {
		return -1, bs.ErrNotFound
	}
	return b.Blockstore.GetSize(c1)
}
