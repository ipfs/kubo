package idstore

import (
	"context"
	"fmt"

	cide "github.com/ipfs/go-ipfs/thirdparty/cidextra"

	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	bls "gx/ipfs/QmaG4DZ4JaqEfvPWt5nPPgoTzhc1tr1T3f4Nu9Jpdm8ymY/go-ipfs-blockstore"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	blocks "gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
)

// idstore wraps a BlockStore to add support for identity hashes
type idstore struct {
	bs bls.Blockstore
}

func IdStore(bs bls.Blockstore) bls.Blockstore {
	return &idstore{bs}
}

func extractContents(k *cid.Cid) (bool, []byte, error) {
	dmh, err := mh.Decode(k.Hash())
	if err != nil || dmh.Code != mh.ID {
		return false, nil, nil
	}
	if len(dmh.Digest) > cide.MaxIdHashLen {
		return true, nil, fmt.Errorf("identity hash of %d bytes is longer than maximum size of %d bytes",
			len(dmh.Digest), cide.MaxIdHashLen)
	}
	return true, dmh.Digest, nil
}

func (b *idstore) DeleteBlock(k *cid.Cid) error {
	isId, _, err := extractContents(k)
	if err != nil {
		return err
	}
	// always try to delete the block in case it was added to the
	// blockstore without this wrapper
	err = b.bs.DeleteBlock(k)
	if isId && err == bls.ErrNotFound {
		return nil
	}
	return err
}

func (b *idstore) Has(k *cid.Cid) (bool, error) {
	isId, _, err := extractContents(k)
	if err != nil {
		return false, err
	}
	if isId {
		return true, nil
	}
	return b.bs.Has(k)
}

func (b *idstore) Get(k *cid.Cid) (blocks.Block, error) {
	isId, bdata, err := extractContents(k)
	if err != nil {
		return nil, err
	}
	if isId {
		return blocks.NewBlockWithCid(bdata, k)
	}
	return b.bs.Get(k)
}

func (b *idstore) Put(bl blocks.Block) error {
	isId, _, err := extractContents(bl.Cid())
	if err != nil {
		return err
	}
	if isId {
		return nil
	}
	return b.bs.Put(bl)
}

func (b *idstore) PutMany(bs []blocks.Block) error {
	toPut := make([]blocks.Block, 0, len(bs))
	for _, bl := range bs {
		isId, _, err := extractContents(bl.Cid())
		if err != nil {
			return err
		}
		if isId {
			continue
		}
		toPut = append(toPut, bl)
	}
	err := b.bs.PutMany(toPut)
	if err != nil {
		return err
	}
	return nil
}

func (b *idstore) HashOnRead(enabled bool) {
	b.bs.HashOnRead(enabled)
}

func (b *idstore) AllKeysChan(ctx context.Context) (<-chan *cid.Cid, error) {
	return b.bs.AllKeysChan(ctx)
}
