package helpers

import (
	"context"
	"fmt"
	"os"

	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	pi "gx/ipfs/Qmb3jLEFAQrqdVgWUajqEyuuDoavkSq1XQXz6tWdFWF995/go-ipfs-posinfo"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// BlockSizeLimit specifies the maximum size an imported block can have.
var BlockSizeLimit = 1048576 // 1 MB

// rough estimates on expected sizes
var roughLinkBlockSize = 1 << 13 // 8KB
var roughLinkSize = 34 + 8 + 5   // sha256 multihash + size + no name + protobuf framing

// DefaultLinksPerBlock governs how the importer decides how many links there
// will be per block. This calculation is based on expected distributions of:
//  * the expected distribution of block sizes
//  * the expected distribution of link sizes
//  * desired access speed
// For now, we use:
//
//   var roughLinkBlockSize = 1 << 13 // 8KB
//   var roughLinkSize = 288          // sha256 + framing + name
//   var DefaultLinksPerBlock = (roughLinkBlockSize / roughLinkSize)
//
// See calc_test.go
var DefaultLinksPerBlock = (roughLinkBlockSize / roughLinkSize)

// ErrSizeLimitExceeded signals that a block is larger than BlockSizeLimit.
var ErrSizeLimitExceeded = fmt.Errorf("object size limit exceeded")

// UnixfsNode is a struct created to aid in the generation
// of unixfs DAG trees
type UnixfsNode struct {
	raw     bool
	rawnode *dag.RawNode
	node    *dag.ProtoNode
	ufmt    *ft.FSNode
	posInfo *pi.PosInfo
}

// NewUnixfsNodeFromDag reconstructs a Unixfs node from a given dag node
func NewUnixfsNodeFromDag(nd *dag.ProtoNode) (*UnixfsNode, error) {
	mb, err := ft.FSNodeFromBytes(nd.Data())
	if err != nil {
		return nil, err
	}

	return &UnixfsNode{
		node: nd,
		ufmt: mb,
	}, nil
}

// SetPrefix sets the CID Prefix
func (n *UnixfsNode) SetPrefix(prefix *cid.Prefix) {
	n.node.SetPrefix(prefix)
}

// NumChildren returns the number of children referenced by this UnixfsNode.
func (n *UnixfsNode) NumChildren() int {
	return n.ufmt.NumChildren()
}

// Set replaces the current UnixfsNode with another one. It performs
// a shallow copy.
func (n *UnixfsNode) Set(other *UnixfsNode) {
	n.node = other.node
	n.raw = other.raw
	n.rawnode = other.rawnode
	if other.ufmt != nil {
		n.ufmt.Data = other.ufmt.Data
	}
}

// GetChild gets the ith child of this node from the given DAGService.
func (n *UnixfsNode) GetChild(ctx context.Context, i int, ds ipld.DAGService) (*UnixfsNode, error) {
	nd, err := n.node.Links()[i].GetNode(ctx, ds)
	if err != nil {
		return nil, err
	}

	pbn, ok := nd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	return NewUnixfsNodeFromDag(pbn)
}

// AddChild adds the given UnixfsNode as a child of the receiver.
// The passed in DagBuilderHelper is used to store the child node an
// pin it locally so it doesnt get lost.
func (n *UnixfsNode) AddChild(child *UnixfsNode, db *DagBuilderHelper) error {
	n.ufmt.AddBlockSize(child.FileSize())

	childnode, err := child.GetDagNode()
	if err != nil {
		return err
	}

	// Add a link to this node without storing a reference to the memory
	// This way, we avoid nodes building up and consuming all of our RAM
	err = n.node.AddNodeLinkClean("", childnode)
	if err != nil {
		return err
	}

	err = db.batch.Add(childnode)

	return err
}

// RemoveChild deletes the child node at the given index.
func (n *UnixfsNode) RemoveChild(index int, dbh *DagBuilderHelper) {
	n.ufmt.RemoveBlockSize(index)
	n.node.SetLinks(append(n.node.Links()[:index], n.node.Links()[index+1:]...))
}

// SetData stores data in this node.
func (n *UnixfsNode) SetData(data []byte) {
	n.ufmt.Data = data
}

// FileSize returns the total file size of this tree (including children)
// In the case of raw nodes, it returns the length of the
// raw data.
func (n *UnixfsNode) FileSize() uint64 {
	if n.raw {
		return uint64(len(n.rawnode.RawData()))
	}
	return n.ufmt.FileSize()
}

// SetPosInfo sets information about the offset of the data of this node in a
// filesystem file.
func (n *UnixfsNode) SetPosInfo(offset uint64, fullPath string, stat os.FileInfo) {
	n.posInfo = &pi.PosInfo{
		Offset:   offset,
		FullPath: fullPath,
		Stat:     stat,
	}
}

// GetDagNode fills out the proper formatting for the unixfs node
// inside of a DAG node and returns the dag node.
func (n *UnixfsNode) GetDagNode() (ipld.Node, error) {
	nd, err := n.getBaseDagNode()
	if err != nil {
		return nil, err
	}

	if n.posInfo != nil {
		if rn, ok := nd.(*dag.RawNode); ok {
			return &pi.FilestoreNode{
				Node:    rn,
				PosInfo: n.posInfo,
			}, nil
		}
	}

	return nd, nil
}

func (n *UnixfsNode) getBaseDagNode() (ipld.Node, error) {
	if n.raw {
		return n.rawnode, nil
	}

	data, err := n.ufmt.GetBytes()
	if err != nil {
		return nil, err
	}
	n.node.SetData(data)
	return n.node, nil
}
