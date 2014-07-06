package merkledag

import (
	"fmt"
	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
)

// can't use []byte/Multihash for keys :(
// so have to convert Multihash bytes to string
type NodeMap map[u.Key]*Node

// A node in the IPFS Merkle DAG.
// nodes have opaque data and a set of navigable links.
type Node struct {
	Links []*Link
	Data  []byte

	// cache encoded/marshaled value
	encoded []byte
}

// An IPFS Merkle DAG Link
type Link struct {
	// utf string name. should be unique per object
	Name string // utf8

	// cumulative size of target object
	Size uint64

	// multihash of the target object
	Hash mh.Multihash

	// a ptr to the actual node for graph manipulation
	Node *Node
}

func (n *Node) AddNodeLink(name string, that *Node) error {
	s, err := that.Size()
	if err != nil {
		return err
	}

	h, err := that.Multihash()
	if err != nil {
		return err
	}

	n.Links = append(n.Links, &Link{
		Name: name,
		Size: s,
		Hash: h,
	})
	return nil
}

func (n *Node) Size() (uint64, error) {
	b, err := n.Encoded(false)
	if err != nil {
		return 0, err
	}

	s := uint64(len(b))
	for _, l := range n.Links {
		s += l.Size
	}
	return s, nil
}

func (n *Node) Multihash() (mh.Multihash, error) {
	b, err := n.Encoded(false)
	if err != nil {
		return nil, err
	}

	return u.Hash(b)
}

func (n *Node) Key() (u.Key, error) {
	h, err := n.Multihash()
	return u.Key(h), err
}

// An IPFS Merkle DAG service.
// the root is virtual (like a forest)
// stores nodes' data in a blockService
type DAGService struct {
	Blocks *blocks.BlockService
}

func (n *DAGService) Put(nd *Node) (u.Key, error) {
	if n == nil {
		return "", fmt.Errorf("DAGService is nil")
	}

	d, err := nd.Encoded(false)
	if err != nil {
		return "", err
	}

	b, err := blocks.NewBlock(d)
	if err != nil {
		return "", err
	}

	return n.Blocks.AddBlock(b)
}

func (n *DAGService) Get(k u.Key) (*Node, error) {
	if n == nil {
		return nil, fmt.Errorf("DAGService is nil")
	}

	b, err := n.Blocks.GetBlock(k)
	if err != nil {
		return nil, err
	}

	return Decoded(b.Data)
}
