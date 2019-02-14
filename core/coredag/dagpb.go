package coredag

import (
	"io"
	"io/ioutil"
	"math"

	"gx/ipfs/QmQvMsV5aPyd7eMd3U1hvAUhZEupG3rXbVZn7ppU5RE6bt/go-merkledag"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"
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

	nd.SetCidBuilder(cidPrefix(mhType, mhLen))

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

	nd.SetCidBuilder(cidPrefix(mhType, mhLen))

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
