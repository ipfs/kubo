package coredag

import (
	"io"
	"io/ioutil"
	"math"

	"github.com/ipfs/go-ipfs/merkledag"

	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

func dagpbJSONParser(r io.Reader, mhType uint64, mhLen int) ([]ipld.Node, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	nd := &merkledag.ProtoNode{}

	err = nd.UnmarshalJSON(data)
	if err != nil {
		return nil, err
	}

	nd.SetPrefix(cidPrefix(mhType, mhLen))

	return []ipld.Node{nd}, nil
}

func dagpbRawParser(r io.Reader, mhType uint64, mhLen int) ([]ipld.Node, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	nd, err := merkledag.DecodeProtobuf(data)
	if err != nil {
		return nil, err
	}

	nd.SetPrefix(cidPrefix(mhType, mhLen))

	return []ipld.Node{nd}, nil
}

func cidPrefix(mhType uint64, mhLen int) *cid.Prefix {
	if mhType == math.MaxUint64 {
		mhType = mh.SHA2_256
	}

	prefix := &cid.Prefix{
		MhType:   mhType,
		MhLength: mhLen,
		Version:  1,
		Codec:    cid.DagProtobuf,
	}

	if mhType == mh.SHA2_256 {
		prefix.Version = 0
	}

	return prefix
}
