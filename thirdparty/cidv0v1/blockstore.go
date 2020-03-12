package cidv0v1

import (
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	bs "github.com/ipfs/go-ipfs-blockstore"
	mh "github.com/multiformats/go-multihash"
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
	c1 := tryOtherCidVersion(c)
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
	c1 := tryOtherCidVersion(c)
	if !c1.Defined() {
		return nil, bs.ErrNotFound
	}
	block, err = b.Blockstore.Get(c1)
	if err != nil {
		return nil, err
	}
	return blocks.NewBlockWithCid(block.RawData(), c)
}

func (b *blockstore) GetSize(c cid.Cid) (int, error) {
	size, err := b.Blockstore.GetSize(c)
	if err == nil {
		return size, nil
	}
	if err != bs.ErrNotFound {
		return -1, err
	}
	c1 := tryOtherCidVersion(c)
	if !c1.Defined() {
		return -1, bs.ErrNotFound
	}
	return b.Blockstore.GetSize(c1)
}

func tryOtherCidVersion(c cid.Cid) cid.Cid {
	prefix := c.Prefix()
	if prefix.Codec != cid.DagProtobuf || prefix.MhType != mh.SHA2_256 || prefix.MhLength != 32 {
		return cid.Undef
	}
	var c1 cid.Cid
	if prefix.Version == 0 {
		c1 = cid.NewCidV1(cid.DagProtobuf, c.Hash())
	} else {
		c1 = cid.NewCidV0(c.Hash())
	}
	return c1
}
