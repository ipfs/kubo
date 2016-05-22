package helpers

import (
	"os"
	"fmt"

	"github.com/ipfs/go-ipfs/commands/files"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

// BlockSizeLimit specifies the maximum size an imported block can have.
var BlockSizeLimit = 1048576 // 1 MB

// rough estimates on expected sizes
var roughDataBlockSize = chunk.DefaultBlockSize
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
	node     *dag.Node
	ufmt     *ft.FSNode
	posInfo  *files.PosInfo
}

// NewUnixfsNode creates a new Unixfs node to represent a file
func NewUnixfsNode() *UnixfsNode {
	return &UnixfsNode{
		node: new(dag.Node),
		ufmt: &ft.FSNode{Type: ft.TFile},
	}
}

// NewUnixfsBlock creates a new Unixfs node to represent a raw data block
func NewUnixfsBlock() *UnixfsNode {
	return &UnixfsNode{
		node: new(dag.Node),
		ufmt: &ft.FSNode{Type: ft.TRaw},
	}
}

// NewUnixfsNodeFromDag reconstructs a Unixfs node from a given dag node
func NewUnixfsNodeFromDag(nd *dag.Node) (*UnixfsNode, error) {
	mb, err := ft.FSNodeFromBytes(nd.Data)
	if err != nil {
		return nil, err
	}

	return &UnixfsNode{
		node: nd,
		ufmt: mb,
	}, nil
}

func (n *UnixfsNode) NumChildren() int {
	return n.ufmt.NumChildren()
}

func (n *UnixfsNode) GetChild(ctx context.Context, i int, ds dag.DAGService) (*UnixfsNode, error) {
	nd, err := n.node.Links[i].GetNode(ctx, ds)
	if err != nil {
		return nil, err
	}

	return NewUnixfsNodeFromDag(nd)
}

// addChild will add the given UnixfsNode as a child of the receiver.
// the passed in DagBuilderHelper is used to store the child node an
// pin it locally so it doesnt get lost
func (n *UnixfsNode) AddChild(child *UnixfsNode, db *DagBuilderHelper) error {
	n.ufmt.AddBlockSize(child.ufmt.FileSize())

	childnode, err := child.GetDagNode(db.needAltData)
	if err != nil {
		return err
	}

	// Add a link to this node without storing a reference to the memory
	// This way, we avoid nodes building up and consuming all of our RAM
	err = n.node.AddNodeLinkClean("", childnode)
	if err != nil {
		return err
	}

	_, err = db.batch.Add(childnode)
	if err != nil {
		return err
	}

	return nil
}

// Removes the child node at the given index
func (n *UnixfsNode) RemoveChild(index int, dbh *DagBuilderHelper) {
	n.ufmt.RemoveBlockSize(index)
	n.node.Links = append(n.node.Links[:index], n.node.Links[index+1:]...)
}

func (n *UnixfsNode) FileSize() uint64 {
	return n.ufmt.FileSize()
}

func (n *UnixfsNode) SetData(data []byte) {
	n.ufmt.Data = data
}

func (n *UnixfsNode) SetPosInfo(offset uint64, fullPath string, stat os.FileInfo) {
	n.posInfo = &files.PosInfo{offset, fullPath, stat}
}

// getDagNode fills out the proper formatting for the unixfs node
// inside of a DAG node and returns the dag node
func (n *UnixfsNode) GetDagNode(needAltData bool) (*dag.Node, error) {
	//fmt.Println("GetDagNode")
	data, err := n.ufmt.GetBytes()
	if err != nil {
		return nil, err
	}
	n.node.Data = data
	if needAltData {
		n.node.DataPtr = n.getAltData()
	}
	return n.node, nil
}

func (n *UnixfsNode) getAltData() (*dag.DataPtr) {
	dp := &dag.DataPtr{PosInfo: n.posInfo, Size: n.ufmt.FileSize()}
	if n.ufmt.NumChildren() == 0 && (n.ufmt.Type == ft.TFile || n.ufmt.Type == ft.TRaw) {
		dp.AltData,_ = n.ufmt.GetBytesNoData()
	}
	return dp
}
