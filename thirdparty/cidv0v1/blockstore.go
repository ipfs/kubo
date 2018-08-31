package cidv0v1

import (
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	blocks "gx/ipfs/QmWAzSEoqZ6xU6pu8yL8e5WaMb7wtbfbhhN4p1DknUPtr3/go-block-format"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	bs "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
)

type blockstore struct {
	bs.Blockstore
}

func NewBlockstore(b bs.Blockstore) bs.Blockstore {
	return &blockstore{b}
}

func (b *blockstore) Has(c *cid.Cid) (bool, error) {
	have, err := b.Blockstore.Has(c)
	if have || err != nil {
		return have, err
	}
	c1 := tryOtherCidVersion(c)
	if c1 == nil {
		return false, nil
	}
	return b.Blockstore.Has(c1)
}

func (b *blockstore) Get(c *cid.Cid) (blocks.Block, error) {
	block, err := b.Blockstore.Get(c)
	if err == nil {
		return block, nil
	}
	if err != bs.ErrNotFound {
		return nil, err
	}
	c1 := tryOtherCidVersion(c)
	if c1 == nil {
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

func (b *blockstore) GetSize(c *cid.Cid) (int, error) {
	size, err := b.Blockstore.GetSize(c)
	if err == nil {
		return size, nil
	}
	if err != bs.ErrNotFound {
		return -1, err
	}
	c1 := tryOtherCidVersion(c)
	if c1 == nil {
		return -1, bs.ErrNotFound
	}
	return b.Blockstore.GetSize(c1)
}

func tryOtherCidVersion(c *cid.Cid) *cid.Cid {
	prefix := c.Prefix()
	if prefix.Codec != cid.DagProtobuf || prefix.MhType != mh.SHA2_256 || prefix.MhLength != 32 {
		return nil
	}
	var c1 *cid.Cid
	if prefix.Version == 0 {
		c1 = cid.NewCidV1(cid.DagProtobuf, c.Hash())
	} else {
		c1 = cid.NewCidV0(c.Hash())
	}
	return c1
}
