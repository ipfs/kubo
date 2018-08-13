package coredag

import (
	"io"
	"io/ioutil"
	"math"

	"gx/ipfs/QmYxX4VfVcxmfsj8U6T5kVtFvHsSidy9tmPyPTW5fy7H3q/go-merkledag"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	ipld "gx/ipfs/QmUSyMZ8Vt4vTZr5HdDEgEfpwAXfQRuDdfCFTt7XBzhxpQ/go-ipld-format"
	block "gx/ipfs/QmZXvzTJTiN6p469osBUtEwm4WwhXXoWcHC8aTS1cAJkjy/go-block-format"
	cid "gx/ipfs/Qmdu2AYUV7yMoVBQPxXNfe7FJcdx16kYtsx6jAPKWQYF1y/go-cid"
)

func rawRawParser(r io.Reader, mhType uint64, mhLen int) ([]ipld.Node, error) {
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
	return []ipld.Node{nd}, nil
}
