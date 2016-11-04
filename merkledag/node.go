package merkledag

import (
	"context"
	"fmt"

	node "gx/ipfs/QmU7bFWQ793qmvNy7outdCaMfSDNk8uqhx4VNrxYj5fj5g/go-ipld-node"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	key "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"
)

var ErrNotProtobuf = fmt.Errorf("expected protobuf dag node")
var ErrLinkNotFound = fmt.Errorf("no link by that name")

// Node represents a node in the IPFS Merkle DAG.
// nodes have opaque data and a set of navigable links.
type ProtoNode struct {
	links []*node.Link
	data  []byte

	// cache encoded/marshaled value
	encoded []byte

	cached *cid.Cid
}

type LinkSlice []*node.Link

func (ls LinkSlice) Len() int           { return len(ls) }
func (ls LinkSlice) Swap(a, b int)      { ls[a], ls[b] = ls[b], ls[a] }
func (ls LinkSlice) Less(a, b int) bool { return ls[a].Name < ls[b].Name }

func NodeWithData(d []byte) *ProtoNode {
	return &ProtoNode{data: d}
}

// AddNodeLink adds a link to another node.
func (n *ProtoNode) AddNodeLink(name string, that node.Node) error {
	n.encoded = nil

	lnk, err := node.MakeLink(that)
	if err != nil {
		return err
	}

	lnk.Name = name

	n.AddRawLink(name, lnk)

	return nil
}

// AddNodeLinkClean adds a link to another node. without keeping a reference to
// the child node
func (n *ProtoNode) AddNodeLinkClean(name string, that node.Node) error {
	n.encoded = nil
	lnk, err := node.MakeLink(that)
	if err != nil {
		return err
	}
	n.AddRawLink(name, lnk)

	return nil
}

// AddRawLink adds a copy of a link to this node
func (n *ProtoNode) AddRawLink(name string, l *node.Link) error {
	n.encoded = nil
	n.links = append(n.links, &node.Link{
		Name: name,
		Size: l.Size,
		Cid:  l.Cid,
	})

	return nil
}

// Remove a link on this node by the given name
func (n *ProtoNode) RemoveNodeLink(name string) error {
	n.encoded = nil
	good := make([]*node.Link, 0, len(n.links))
	var found bool

	for _, l := range n.links {
		if l.Name != name {
			good = append(good, l)
		} else {
			found = true
		}
	}
	n.links = good

	if !found {
		return ErrNotFound
	}

	return nil
}

// Return a copy of the link with given name
func (n *ProtoNode) GetNodeLink(name string) (*node.Link, error) {
	for _, l := range n.links {
		if l.Name == name {
			return &node.Link{
				Name: l.Name,
				Size: l.Size,
				Cid:  l.Cid,
			}, nil
		}
	}
	return nil, ErrLinkNotFound
}

func (n *ProtoNode) GetLinkedProtoNode(ctx context.Context, ds DAGService, name string) (*ProtoNode, error) {
	nd, err := n.GetLinkedNode(ctx, ds, name)
	if err != nil {
		return nil, err
	}

	pbnd, ok := nd.(*ProtoNode)
	if !ok {
		return nil, ErrNotProtobuf
	}

	return pbnd, nil
}

func (n *ProtoNode) GetLinkedNode(ctx context.Context, ds DAGService, name string) (node.Node, error) {
	lnk, err := n.GetNodeLink(name)
	if err != nil {
		return nil, err
	}

	return lnk.GetNode(ctx, ds)
}

// Copy returns a copy of the node.
// NOTE: Does not make copies of Node objects in the links.
func (n *ProtoNode) Copy() *ProtoNode {
	nnode := new(ProtoNode)
	if len(n.data) > 0 {
		nnode.data = make([]byte, len(n.data))
		copy(nnode.data, n.data)
	}

	if len(n.links) > 0 {
		nnode.links = make([]*node.Link, len(n.links))
		copy(nnode.links, n.links)
	}
	return nnode
}

func (n *ProtoNode) RawData() []byte {
	out, _ := n.EncodeProtobuf(false)
	return out
}

func (n *ProtoNode) Data() []byte {
	return n.data
}

func (n *ProtoNode) SetData(d []byte) {
	n.encoded = nil
	n.cached = nil
	n.data = d
}

// UpdateNodeLink return a copy of the node with the link name set to point to
// that. If a link of the same name existed, it is removed.
func (n *ProtoNode) UpdateNodeLink(name string, that *ProtoNode) (*ProtoNode, error) {
	newnode := n.Copy()
	err := newnode.RemoveNodeLink(name)
	err = nil // ignore error
	err = newnode.AddNodeLink(name, that)
	return newnode, err
}

// Size returns the total size of the data addressed by node,
// including the total sizes of references.
func (n *ProtoNode) Size() (uint64, error) {
	b, err := n.EncodeProtobuf(false)
	if err != nil {
		return 0, err
	}

	s := uint64(len(b))
	for _, l := range n.links {
		s += l.Size
	}
	return s, nil
}

// Stat returns statistics on the node.
func (n *ProtoNode) Stat() (*node.NodeStat, error) {
	enc, err := n.EncodeProtobuf(false)
	if err != nil {
		return nil, err
	}

	cumSize, err := n.Size()
	if err != nil {
		return nil, err
	}

	return &node.NodeStat{
		Hash:           n.Key().B58String(),
		NumLinks:       len(n.links),
		BlockSize:      len(enc),
		LinksSize:      len(enc) - len(n.data), // includes framing.
		DataSize:       len(n.data),
		CumulativeSize: int(cumSize),
	}, nil
}

func (n *ProtoNode) Key() key.Key {
	return key.Key(n.Multihash())
}

func (n *ProtoNode) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"node": n.String(),
	}
}

func (n *ProtoNode) Cid() *cid.Cid {
	h := n.Multihash()

	return cid.NewCidV0(h)
}

func (n *ProtoNode) String() string {
	return n.Cid().String()
}

// Multihash hashes the encoded data of this node.
func (n *ProtoNode) Multihash() mh.Multihash {
	// NOTE: EncodeProtobuf generates the hash and puts it in n.cached.
	_, err := n.EncodeProtobuf(false)
	if err != nil {
		// Note: no possibility exists for an error to be returned through here
		panic(err)
	}

	return n.cached.Hash()
}

func (n *ProtoNode) Links() []*node.Link {
	return n.links
}

func (n *ProtoNode) SetLinks(links []*node.Link) {
	n.links = links
}

func (n *ProtoNode) Resolve(path []string) (interface{}, []string, error) {
	return n.ResolveLink(path)
}

func (n *ProtoNode) ResolveLink(path []string) (*node.Link, []string, error) {
	if len(path) == 0 {
		return nil, nil, fmt.Errorf("end of path, no more links to resolve")
	}

	lnk, err := n.GetNodeLink(path[0])
	if err != nil {
		return nil, nil, err
	}

	return lnk, path[1:], nil
}

func (n *ProtoNode) Tree() []string {
	out := make([]string, 0, len(n.links))
	for _, lnk := range n.links {
		out = append(out, lnk.Name)
	}
	return out
}
