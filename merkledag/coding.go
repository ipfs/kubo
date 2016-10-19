package merkledag

import (
	"fmt"
	"sort"

	pb "github.com/ipfs/go-ipfs/merkledag/pb"

	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"
	node "gx/ipfs/QmZx42H5khbVQhV5odp66TApShV4XCujYazcvYduZ4TroB/go-ipld-node"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
)

// for now, we use a PBNode intermediate thing.
// because native go objects are nice.

// unmarshal decodes raw data into a *Node instance.
// The conversion uses an intermediate PBNode.
func (n *ProtoNode) unmarshal(encoded []byte) error {
	var pbn pb.PBNode
	if err := pbn.Unmarshal(encoded); err != nil {
		return fmt.Errorf("Unmarshal failed. %v", err)
	}

	pbnl := pbn.GetLinks()
	n.links = make([]*node.Link, len(pbnl))
	for i, l := range pbnl {
		n.links[i] = &node.Link{Name: l.GetName(), Size: l.GetTsize()}
		c, err := cid.Cast(l.GetHash())
		if err != nil {
			return fmt.Errorf("Link hash #%d is not valid multihash. %v", i, err)
		}
		n.links[i].Cid = c
	}
	sort.Stable(LinkSlice(n.links)) // keep links sorted

	n.data = pbn.GetData()
	n.encoded = encoded
	return nil
}

// Marshal encodes a *Node instance into a new byte slice.
// The conversion uses an intermediate PBNode.
func (n *ProtoNode) Marshal() ([]byte, error) {
	pbn := n.getPBNode()
	data, err := pbn.Marshal()
	if err != nil {
		return data, fmt.Errorf("Marshal failed. %v", err)
	}
	return data, nil
}

func (n *ProtoNode) getPBNode() *pb.PBNode {
	pbn := &pb.PBNode{}
	if len(n.links) > 0 {
		pbn.Links = make([]*pb.PBLink, len(n.links))
	}

	sort.Stable(LinkSlice(n.links)) // keep links sorted
	for i, l := range n.links {
		pbn.Links[i] = &pb.PBLink{}
		pbn.Links[i].Name = &l.Name
		pbn.Links[i].Tsize = &l.Size
		pbn.Links[i].Hash = l.Cid.Bytes()
	}

	if len(n.data) > 0 {
		pbn.Data = n.data
	}
	return pbn
}

// EncodeProtobuf returns the encoded raw data version of a Node instance.
// It may use a cached encoded version, unless the force flag is given.
func (n *ProtoNode) EncodeProtobuf(force bool) ([]byte, error) {
	sort.Stable(LinkSlice(n.links)) // keep links sorted
	if n.encoded == nil || force {
		n.cached = nil
		var err error
		n.encoded, err = n.Marshal()
		if err != nil {
			return nil, err
		}
	}

	if n.cached == nil {
		n.cached = cid.NewCidV0(u.Hash(n.encoded))
	}

	return n.encoded, nil
}

// Decoded decodes raw data and returns a new Node instance.
func DecodeProtobuf(encoded []byte) (*ProtoNode, error) {
	n := new(ProtoNode)
	err := n.unmarshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("incorrectly formatted merkledag node: %s", err)
	}
	return n, nil
}
