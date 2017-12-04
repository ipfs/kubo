package coredag

import (
	"io"
	"io/ioutil"
	"math"

	"github.com/ipfs/go-ipfs/merkledag"

	node "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
	mh "gx/ipfs/QmYeKnKpubCMRiq3PGZcTREErthbb5Q9cXsCoSkD9bjEBd/go-multihash"
	block "gx/ipfs/QmYsEQydGrsxNZfAiskvQ76N2xE9hDQtSAkRSynwMiUK3c/go-block-format"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

func rawRawParser(r io.Reader, mhType uint64, mhLen int) ([]node.Node, error) {
	if mhType == math.MaxUint64 {
		mhType = mh.SHA2_256
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	h, err := mh.Sum(data, mhType, mhLen)
	if err != nil {
		return nil, err
	}
	c := cid.NewCidV1(cid.Raw, h)
	blk, err := block.NewBlockWithCid(data, c)
	if err != nil {
		return nil, err
	}
	nd := &merkledag.RawNode{Block: blk}
	return []node.Node{nd}, nil
}
