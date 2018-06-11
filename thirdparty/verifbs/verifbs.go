package verifbs

import (
	"github.com/ipfs/go-ipfs/thirdparty/verifcid"

	blocks "gx/ipfs/QmTRCUvZLiir12Qr6MV3HKfKMHX8Nf1Vddn6t2g5nsQSb9/go-block-format"
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
	bstore "gx/ipfs/QmbaPGg81pvQiC5vTXtC9Jo8rdrWUjRaugH71WYNsgi6Ev/go-ipfs-blockstore"
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
