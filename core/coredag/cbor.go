package coredag

import (
	"io"
	"io/ioutil"

	node "gx/ipfs/QmRL2JDEtNzSkEjMgsUBXgmHKeJ7a4V6QoirXHrc93igo2/go-ipld-format"
	ipldcbor "gx/ipfs/QmSKrDrzpjRTiq4EK7puuNr5Rvr6Yu8Yon9fCZuK2obLYx/go-ipld-cbor"
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
