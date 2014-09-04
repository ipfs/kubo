package merkledag

import (
	"fmt"

	blocks "github.com/jbenet/go-ipfs/blocks"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
)

// NodeMap maps u.Keys to Nodes.
// We cannot use []byte/Multihash for keys :(
// so have to convert Multihash bytes to string (u.Key)
type NodeMap map[u.Key]*Node

// Node represents a node in the IPFS Merkle DAG.
// nodes have opaque data and a set of navigable links.
type Node struct {
	Links []*Link
	Data  []byte

	// cache encoded/marshaled value
	encoded []byte
}

// Link represents an IPFS Merkle DAG Link between Nodes.
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

// AddNodeLink adds a link to another node.
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
		Node: that,
	})
	return nil
}

// Size returns the total size of the data addressed by node,
// including the total sizes of references.
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

// Multihash hashes the encoded data of this node.
func (n *Node) Multihash() (mh.Multihash, error) {
	b, err := n.Encoded(false)
	if err != nil {
		return nil, err
	}

	return u.Hash(b)
}

// Key returns the Multihash as a key, for maps.
func (n *Node) Key() (u.Key, error) {
	h, err := n.Multihash()
	return u.Key(h), err
}

// DAGService is an IPFS Merkle DAG service.
// - the root is virtual (like a forest)
// - stores nodes' data in a BlockService
type DAGService struct {
	Blocks *bserv.BlockService
}

// Add adds a node to the DAGService, storing the block in the BlockService
func (n *DAGService) Add(nd *Node) (u.Key, error) {
	k, _ := nd.Key()
	u.DOut("DagService Add [%s]\n", k.Pretty())
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

func (n *DAGService) AddRecursive(nd *Node) error {
	_, err := n.Add(nd)
	if err != nil {
		return err
	}

	for _, link := range nd.Links {
		fmt.Println("Adding link.")
		if link.Node == nil {
			panic("Why does this node have a nil link?\n")
		}
		err := n.AddRecursive(link.Node)
		if err != nil {
			return err
		}
	}

	return nil
}

// Get retrieves a node from the DAGService, fetching the block in the BlockService
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
