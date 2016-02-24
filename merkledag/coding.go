package merkledag

import (
	"fmt"
	"sort"

	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"

	pb "github.com/ipfs/go-ipfs/merkledag/pb"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

// for now, we use a PBNode intermediate thing.
// because native go objects are nice.

// Unmarshal decodes raw data into a *Node instance.
// The conversion uses an intermediate PBNode.
func (n *Node) Unmarshal(encoded []byte) error {
	var pbn pb.PBNode
	if err := pbn.Unmarshal(encoded); err != nil {
		return fmt.Errorf("Unmarshal failed. %v", err)
	}

	pbnl := pbn.GetLinks()
	n.Links = make([]*Link, len(pbnl))
	for i, l := range pbnl {
		n.Links[i] = &Link{Name: l.GetName(), Size: l.GetTsize()}
		h, err := mh.Cast(l.GetHash())
		if err != nil {
			return fmt.Errorf("Link hash is not valid multihash. %v", err)
		}
		n.Links[i].Hash = h
	}
	if !n.sorted {
		sort.Stable(LinkSlice(n.Links)) // keep links sorted
		n.sorted = true
	}

	n.Data = pbn.GetData()
	return nil
}

// Marshal encodes a *Node instance into a new byte slice.
// The conversion uses an intermediate PBNode.
func (n *Node) Marshal() ([]byte, error) {
	pbn := n.getPBNode()
	data, err := pbn.Marshal()
	if err != nil {
		return data, fmt.Errorf("Marshal failed. %v", err)
	}
	return data, nil
}

func (n *Node) getPBNode() *pb.PBNode {
	pbn := &pb.PBNode{}
	if len(n.Links) > 0 {
		pbn.Links = make([]*pb.PBLink, len(n.Links))
	}

	if !n.sorted {
		sort.Stable(LinkSlice(n.Links)) // keep links sorted
		n.sorted = true
	}
	for i, l := range n.Links {
		pbn.Links[i] = &pb.PBLink{}
		pbn.Links[i].Name = &l.Name
		pbn.Links[i].Tsize = &l.Size
		pbn.Links[i].Hash = []byte(l.Hash)
	}

	if len(n.Data) > 0 {
		pbn.Data = n.Data
	}
	return pbn
}

// Encoded returns the encoded raw data version of a Node instance.
// It may use a cached encoded version, unless the force flag is given.
func (n *Node) Encoded(force bool) ([]byte, error) {
	if !n.sorted {
		sort.Stable(LinkSlice(n.Links)) // keep links sorted
		n.sorted = true
	}
	if n.encoded == nil || force {
		var err error
		n.encoded, err = n.Marshal()
		if err != nil {
			return nil, err
		}
		n.cached = u.Hash(n.encoded)
	}

	return n.encoded, nil
}

// Decoded decodes raw data and returns a new Node instance.
func Decoded(encoded []byte) (*Node, error) {
	n := new(Node)
	err := n.Unmarshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("incorrectly formatted merkledag node: %s", err)
	}
	return n, nil
}
