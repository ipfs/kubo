package coredag

import (
	"io"
	"io/ioutil"

	node "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
	ipldcbor "gx/ipfs/QmeZv9VXw2SfVbX55LV6kGTWASKBc9ZxAVqGBeJcDGdoXy/go-ipld-cbor"
)

func cborJSONParser(r io.Reader, mhType uint64, mhLen int) ([]node.Node, error) {
	nd, err := ipldcbor.FromJson(r, mhType, mhLen)
	if err != nil {
		return nil, err
	}

	return []node.Node{nd}, nil
}

func cborRawParser(r io.Reader, mhType uint64, mhLen int) ([]node.Node, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	nd, err := ipldcbor.Decode(data, mhType, mhLen)
	if err != nil {
		return nil, err
	}

	return []node.Node{nd}, nil
}
