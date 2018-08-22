package verifbs

import (
	"gx/ipfs/QmVUhfewLZpSaAiBYCpw2krYMaiVmFuhr2iurQLuRoU6sD/go-verifcid"
	blocks "gx/ipfs/QmWAzSEoqZ6xU6pu8yL8e5WaMb7wtbfbhhN4p1DknUPtr3/go-block-format"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
)

type VerifBSGC struct {
	bstore.GCBlockstore
}

func (bs *VerifBSGC) Put(b blocks.Block) error {
	if err := verifcid.ValidateCid(b.Cid()); err != nil {
		return err
	}
	return bs.GCBlockstore.Put(b)
}

func (bs *VerifBSGC) PutMany(blks []blocks.Block) error {
	for _, b := range blks {
		if err := verifcid.ValidateCid(b.Cid()); err != nil {
			return err
		}
	}
	return bs.GCBlockstore.PutMany(blks)
}

func (bs *VerifBSGC) Get(c *cid.Cid) (blocks.Block, error) {
	if err := verifcid.ValidateCid(c); err != nil {
		return nil, err
	}
	return bs.GCBlockstore.Get(c)
}

type VerifBS struct {
	bstore.Blockstore
}

func (bs *VerifBS) Put(b blocks.Block) error {
	if err := verifcid.ValidateCid(b.Cid()); err != nil {
		return err
	}
	return bs.Blockstore.Put(b)
}

func (bs *VerifBS) PutMany(blks []blocks.Block) error {
	for _, b := range blks {
		if err := verifcid.ValidateCid(b.Cid()); err != nil {
			return err
		}
	}
	return bs.Blockstore.PutMany(blks)
}

func (bs *VerifBS) Get(c *cid.Cid) (blocks.Block, error) {
	if err := verifcid.ValidateCid(c); err != nil {
		return nil, err
	}
	return bs.Blockstore.Get(c)
}
