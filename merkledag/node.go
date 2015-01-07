package merkledag

import (
	"fmt"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	u "github.com/jbenet/go-ipfs/util"
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

	cached mh.Multihash
}

// NodeStat is a statistics object for a Node. Mostly sizes.
type NodeStat struct {
	NumLinks       int // number of links in link table
	BlockSize      int // size of the raw data
	LinksSize      int // size of the links segment
	DataSize       int // size of the data segment
	CumulativeSize int // cumulatie size of object + all it references
}

func (ns NodeStat) String() string {
	f := "NodeStat{NumLinks: %d, BlockSize: %d, LinksSize: %d, DataSize: %d, CumulativeSize: %d}"
	return fmt.Sprintf(f, ns.NumLinks, ns.BlockSize, ns.LinksSize, ns.DataSize, ns.CumulativeSize)
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

type LinkSlice []*Link

func (ls LinkSlice) Len() int           { return len(ls) }
func (ls LinkSlice) Swap(a, b int)      { ls[a], ls[b] = ls[b], ls[a] }
func (ls LinkSlice) Less(a, b int) bool { return ls[a].Name < ls[b].Name }

// MakeLink creates a link to the given node
func MakeLink(n *Node) (*Link, error) {
	s, err := n.Size()
	if err != nil {
		return nil, err
	}

	h, err := n.Multihash()
	if err != nil {
		return nil, err
	}
	return &Link{
		Size: s,
		Hash: h,
	}, nil
}

// GetNode returns the MDAG Node that this link points to
func (l *Link) GetNode(serv DAGService) (*Node, error) {
	if l.Node != nil {
		return l.Node, nil
	}

	return serv.Get(u.Key(l.Hash))
}

// AddNodeLink adds a link to another node.
func (n *Node) AddNodeLink(name string, that *Node) error {
	lnk, err := MakeLink(that)
	if err != nil {
		return err
	}
	lnk.Name = name
	lnk.Node = that

	n.Links = append(n.Links, lnk)
	return nil
}

// AddNodeLink adds a link to another node. without keeping a reference to
// the child node
func (n *Node) AddNodeLinkClean(name string, that *Node) error {
	lnk, err := MakeLink(that)
	if err != nil {
		return err
	}
	lnk.Name = name

	n.Links = append(n.Links, lnk)
	return nil
}

// Remove a link on this node by the given name
func (n *Node) RemoveNodeLink(name string) error {
	for i, l := range n.Links {
		if l.Name == name {
			n.Links = append(n.Links[:i], n.Links[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

// Copy returns a copy of the node.
// NOTE: does not make copies of Node objects in the links.
func (n *Node) Copy() *Node {
	nnode := new(Node)
	nnode.Data = make([]byte, len(n.Data))
	copy(nnode.Data, n.Data)

	nnode.Links = make([]*Link, len(n.Links))
	copy(nnode.Links, n.Links)
	return nnode
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

// Stat returns statistics on the node.
func (n *Node) Stat() (NodeStat, error) {
	enc, err := n.Encoded(false)
	if err != nil {
		return NodeStat{}, err
	}

	cumSize, err := n.Size()
	if err != nil {
		return NodeStat{}, err
	}

	return NodeStat{
		NumLinks:       len(n.Links),
		BlockSize:      len(enc),
		LinksSize:      len(enc) - len(n.Data), // includes framing.
		DataSize:       len(n.Data),
		CumulativeSize: int(cumSize),
	}, nil
}

// Multihash hashes the encoded data of this node.
func (n *Node) Multihash() (mh.Multihash, error) {
	// Note: Encoded generates the hash and puts it in n.cached.
	_, err := n.Encoded(false)
	if err != nil {
		return nil, err
	}

	return n.cached, nil
}

// Key returns the Multihash as a key, for maps.
func (n *Node) Key() (u.Key, error) {
	h, err := n.Multihash()
	return u.Key(h), err
}
