package merkledag

import (
	"fmt"

	"context"

	"github.com/ipfs/go-ipfs/commands/files"

	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"
	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	key "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"
)

var ErrLinkNotFound = fmt.Errorf("no link by that name")

// Node represents a node in the IPFS Merkle DAG.
// nodes have opaque data and a set of navigable links.
type Node struct {
	Links []*Link
	data  []byte

	// cache encoded/marshaled value
	encoded []byte

	cached *cid.Cid

	posInfo *files.PosInfo
}

// NodeStat is a statistics object for a Node. Mostly sizes.
type NodeStat struct {
	Hash           string
	NumLinks       int // number of links in link table
	BlockSize      int // size of the raw, encoded data
	LinksSize      int // size of the links segment
	DataSize       int // size of the data segment
	CumulativeSize int // cumulative size of object and its references
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

	h := n.Multihash()

	return &Link{
		Size: s,
		Hash: h,
	}, nil
}

// GetNode returns the MDAG Node that this link points to
func (l *Link) GetNode(ctx context.Context, serv DAGService) (*Node, error) {
	return serv.Get(ctx, legacyCidFromLink(l))
}

func NodeWithData(d []byte) *Node {
	return &Node{data: d}
}

// AddNodeLink adds a link to another node.
func (n *Node) AddNodeLink(name string, that *Node) error {
	n.encoded = nil

	lnk, err := MakeLink(that)

	lnk.Name = name
	if err != nil {
		return err
	}

	n.AddRawLink(name, lnk)

	return nil
}

// AddNodeLinkClean adds a link to another node. without keeping a reference to
// the child node
func (n *Node) AddNodeLinkClean(name string, that *Node) error {
	n.encoded = nil
	lnk, err := MakeLink(that)
	if err != nil {
		return err
	}
	n.AddRawLink(name, lnk)

	return nil
}

// AddRawLink adds a copy of a link to this node
func (n *Node) AddRawLink(name string, l *Link) error {
	n.encoded = nil
	n.Links = append(n.Links, &Link{
		Name: name,
		Size: l.Size,
		Hash: l.Hash,
	})

	return nil
}

// Remove a link on this node by the given name
func (n *Node) RemoveNodeLink(name string) error {
	n.encoded = nil
	good := make([]*Link, 0, len(n.Links))
	var found bool

	for _, l := range n.Links {
		if l.Name != name {
			good = append(good, l)
		} else {
			found = true
		}
	}
	n.Links = good

	if !found {
		return ErrNotFound
	}

	return nil
}

// Return a copy of the link with given name
func (n *Node) GetNodeLink(name string) (*Link, error) {
	for _, l := range n.Links {
		if l.Name == name {
			return &Link{
				Name: l.Name,
				Size: l.Size,
				Hash: l.Hash,
			}, nil
		}
	}
	return nil, ErrLinkNotFound
}

func (n *Node) GetLinkedNode(ctx context.Context, ds DAGService, name string) (*Node, error) {
	lnk, err := n.GetNodeLink(name)
	if err != nil {
		return nil, err
	}

	return lnk.GetNode(ctx, ds)
}

// Copy returns a copy of the node.
// NOTE: Does not make copies of Node objects in the links.
func (n *Node) Copy() *Node {
	nnode := new(Node)
	if len(n.data) > 0 {
		nnode.data = make([]byte, len(n.data))
		copy(nnode.data, n.data)
	}

	if len(n.Links) > 0 {
		nnode.Links = make([]*Link, len(n.Links))
		copy(nnode.Links, n.Links)
	}
	return nnode
}

func (n *Node) RawData() []byte {
	out, _ := n.EncodeProtobuf(false)
	return out
}

func (n *Node) PosInfo() *files.PosInfo {
	return n.posInfo
}

func (n *Node) Data() []byte {
	return n.data
}

func (n *Node) SetData(d []byte) {
	n.encoded = nil
	n.cached = nil
	n.data = d
}

func (n *Node) SetPosInfo(pi *files.PosInfo) {
	n.posInfo = pi
}

// UpdateNodeLink return a copy of the node with the link name set to point to
// that. If a link of the same name existed, it is removed.
func (n *Node) UpdateNodeLink(name string, that *Node) (*Node, error) {
	newnode := n.Copy()
	err := newnode.RemoveNodeLink(name)
	err = nil // ignore error
	err = newnode.AddNodeLink(name, that)
	return newnode, err
}

// Size returns the total size of the data addressed by node,
// including the total sizes of references.
func (n *Node) Size() (uint64, error) {
	b, err := n.EncodeProtobuf(false)
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
func (n *Node) Stat() (*NodeStat, error) {
	enc, err := n.EncodeProtobuf(false)
	if err != nil {
		return nil, err
	}

	cumSize, err := n.Size()
	if err != nil {
		return nil, err
	}

	return &NodeStat{
		Hash:           n.Key().B58String(),
		NumLinks:       len(n.Links),
		BlockSize:      len(enc),
		LinksSize:      len(enc) - len(n.data), // includes framing.
		DataSize:       len(n.data),
		CumulativeSize: int(cumSize),
	}, nil
}

func (n *Node) Key() key.Key {
	return key.Key(n.Multihash())
}

func (n *Node) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"node": n.String(),
	}
}

func (n *Node) Cid() *cid.Cid {
	h := n.Multihash()

	return cid.NewCidV0(h)
}

func (n *Node) String() string {
	return n.Cid().String()
}

// Multihash hashes the encoded data of this node.
func (n *Node) Multihash() mh.Multihash {
	// NOTE: EncodeProtobuf generates the hash and puts it in n.cached.
	_, err := n.EncodeProtobuf(false)
	if err != nil {
		// Note: no possibility exists for an error to be returned through here
		panic(err)
	}

	return n.cached.Hash()
}
