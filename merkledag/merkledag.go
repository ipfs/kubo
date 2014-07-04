package merkledag

import (
	mh "github.com/jbenet/go-multihash"
)

// A node in the IPFS Merkle DAG.
// nodes have opaque data and a set of navigable links.
type Node struct {
	Links []*Link
	Data  []byte
}

// An IPFS Merkle DAG Link
type Link struct {
	// utf string name. should be unique per object
	Name string // utf8

	// cumulative size of target object
	Size uint64

	// multihash of the target object
	Hash mh.Multihash
}

type EncodedNode []byte

func (n *Node) Size() (uint64, error) {
	d, err := n.Marshal()
	if err != nil {
		return 0, err
	}

	s := uint64(len(d))
	for _, l := range n.Links {
		s += l.Size
	}
	return s, nil
}
