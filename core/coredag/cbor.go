package coredag

import (
	"io"
	"io/ioutil"

	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	ipldcbor "gx/ipfs/QmepvyyduWnXHm1G7ybmGbJfQQHTAo36DjP2nvF7H7ZXjE/go-ipld-cbor"
)

func cborJSONParser(r io.Reader, mhType uint64, mhLen int) ([]ipld.Node, error) {
	nd, err := ipldcbor.FromJson(r, mhType, mhLen)
	if err != nil {
		return nil, err
	}

	return []ipld.Node{nd}, nil
}

func cborRawParser(r io.Reader, mhType uint64, mhLen int) ([]ipld.Node, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	nd, err := ipldcbor.Decode(data, mhType, mhLen)
	if err != nil {
		return nil, err
	}

	return []ipld.Node{nd}, nil
}
