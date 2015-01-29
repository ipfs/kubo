// package importer implements utilities used to create ipfs DAGs from files
// and readers
package importer

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/pin"
	ft "github.com/jbenet/go-ipfs/unixfs"
	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("importer")

// BlockSizeLimit specifies the maximum size an imported block can have.
var BlockSizeLimit = 1048576 // 1 MB

// rough estimates on expected sizes
var roughDataBlockSize = chunk.DefaultBlockSize
var roughLinkBlockSize = 1 << 13 // 8KB
var roughLinkSize = 258 + 8 + 5  // sha256 multihash + size + no name + protobuf framing

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

// IndirectBlocksCopyData governs whether indirect blocks should copy over
// data from their first child, and how much. If this is 0, indirect blocks
// have no data, only links. If this is larger, Indirect blocks will copy
// as much as (maybe less than) this many bytes.
//
// This number should be <= (BlockSizeLimit - (DefaultLinksPerBlock * LinkSize))
// Note that it is not known here what the LinkSize is, because the hash function
// could vary wildly in size. Exercise caution when setting this option. For
// safety, it will be clipped to (BlockSizeLimit - (DefaultLinksPerBlock * 256))
var IndirectBlockDataSize = 0

// this check is here to ensure the conditions on IndirectBlockDataSize hold.
// returns int because it will be used as an input to `make()` later on. if
// `int` will flip over to negative, better know here.
func defaultIndirectBlockDataSize() int {
	max := BlockSizeLimit - (DefaultLinksPerBlock * 256)
	if IndirectBlockDataSize < max {
		max = IndirectBlockDataSize
	}
	if max < 0 {
		return 0
	}
	return max
}

// Builds a DAG from the given file, writing created blocks to disk as they are
// created
func BuildDagFromFile(fpath string, ds dag.DAGService, mp pin.ManualPinner) (*dag.Node, error) {
	stat, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("`%s` is a directory", fpath)
	}

	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return BuildDagFromReader(f, ds, mp, chunk.DefaultSplitter)
}

// unixfsNode is a struct created to aid in the generation
// of unixfs DAG trees
type unixfsNode struct {
	node *dag.Node
	ufmt *ft.MultiBlock
}

func newUnixfsNode() *unixfsNode {
	return &unixfsNode{
		node: new(dag.Node),
		ufmt: new(ft.MultiBlock),
	}
}

func unixfsNodeFromDagNode(nd *dag.Node) (*unixfsNode, error) {
	un := new(unixfsNode)
	un.node = nd
	ufmt, err := ft.BytesToMultiblock(nd.Data)
	if err != nil {
		return nil, err
	}
	un.ufmt = ufmt
	return un, nil
}

func (n *unixfsNode) numChildren() int {
	return n.ufmt.NumChildren()
}

// addChild will add the given unixfsNode as a child of the receiver.
// the passed in dagBuilderHelper is used to store the child node an
// pin it locally so it doesnt get lost
func (n *unixfsNode) addChild(child *unixfsNode, db *dagBuilderHelper) error {
	n.ufmt.AddBlockSize(child.ufmt.FileSize())

	childnode, err := child.getDagNode()
	if err != nil {
		return err
	}

	// Add a link to this node without storing a reference to the memory
	// This way, we avoid nodes building up and consuming all of our RAM
	err = n.node.AddNodeLinkClean("", childnode)
	if err != nil {
		return err
	}

	childkey, err := db.dserv.Add(childnode)
	if err != nil {
		return err
	}

	// Pin the child node indirectly
	if db.mp != nil {
		db.mp.PinWithMode(childkey, pin.Indirect)
	}

	return nil
}

func (n *unixfsNode) setData(data []byte) {
	n.ufmt.Data = data
}

// getDagNode fills out the proper formatting for the unixfs node
// inside of a DAG node and returns the dag node
func (n *unixfsNode) getDagNode() (*dag.Node, error) {
	data, err := n.ufmt.GetBytes()
	if err != nil {
		return nil, err
	}
	n.node.Data = data
	return n.node, nil
}

func BuildDagFromReader(r io.Reader, ds dag.DAGService, mp pin.ManualPinner, spl chunk.BlockSplitter) (*dag.Node, error) {
	// Start the splitter
	blkch := spl.Split(r)

	// Create our builder helper
	db := &dagBuilderHelper{
		dserv:    ds,
		mp:       mp,
		in:       blkch,
		maxlinks: DefaultLinksPerBlock,
		indrSize: defaultIndirectBlockDataSize(),
	}

	var root *unixfsNode
	for level := 0; !db.done(); level++ {

		nroot := newUnixfsNode()

		// add our old root as a child of the new root.
		if root != nil { // nil if it's the first node.
			if err := nroot.addChild(root, db); err != nil {
				return nil, err
			}
		}

		// fill it up.
		if err := db.fillNodeRec(nroot, level); err != nil {
			return nil, err
		}

		root = nroot
	}
	if root == nil {
		root = newUnixfsNode()
	}

	rootnode, err := root.getDagNode()
	if err != nil {
		return nil, err
	}

	rootkey, err := ds.Add(rootnode)
	if err != nil {
		return nil, err
	}

	if mp != nil {
		mp.PinWithMode(rootkey, pin.Recursive)
		err := mp.Flush()
		if err != nil {
			return nil, err
		}
	}

	return root.getDagNode()
}

// dagBuilderHelper wraps together a bunch of objects needed to
// efficiently create unixfs dag trees
type dagBuilderHelper struct {
	dserv    dag.DAGService
	mp       pin.ManualPinner
	in       <-chan []byte
	nextData []byte // the next item to return.
	maxlinks int
	indrSize int // see IndirectBlockData
}

// prepareNext consumes the next item from the channel and puts it
// in the nextData field. it is idempotent-- if nextData is full
// it will do nothing.
//
// i realized that building the dag becomes _a lot_ easier if we can
// "peek" the "are done yet?" (i.e. not consume it from the channel)
func (db *dagBuilderHelper) prepareNext() {
	if db.in == nil {
		// if our input is nil, there is "nothing to do". we're done.
		// as if there was no data at all. (a sort of zero-value)
		return
	}

	// if we already have data waiting to be consumed, we're ready.
	if db.nextData != nil {
		return
	}

	// if it's closed, nextData will be correctly set to nil, signaling
	// that we're done consuming from the channel.
	db.nextData = <-db.in
}

// done returns whether or not we're done consuming the incoming data.
func (db *dagBuilderHelper) done() bool {
	// ensure we have an accurate perspective on data
	// as `done` this may be called before `next`.
	db.prepareNext() // idempotent
	return db.nextData == nil
}

// next returns the next chunk of data to be inserted into the dag
// if it returns nil, that signifies that the stream is at an end, and
// that the current building operation should finish
func (db *dagBuilderHelper) next() []byte {
	db.prepareNext() // idempotent
	d := db.nextData
	db.nextData = nil // signal we've consumed it
	return d
}

// fillNodeRec will fill the given node with data from the dagBuilders input
// source down to an indirection depth as specified by 'depth'
// it returns the total dataSize of the node, and a potential error
//
// warning: **children** pinned indirectly, but input node IS NOT pinned.
func (db *dagBuilderHelper) fillNodeRec(node *unixfsNode, depth int) error {
	if depth < 0 {
		return errors.New("attempt to fillNode at depth < 0")
	}

	// Base case
	if depth <= 0 { // catch accidental -1's in case error above is removed.
		return db.fillNodeWithData(node)
	}

	// while we have room AND we're not done
	for node.numChildren() < db.maxlinks && !db.done() {
		child := newUnixfsNode()

		if err := db.fillNodeRec(child, depth-1); err != nil {
			return err
		}

		if err := node.addChild(child, db); err != nil {
			return err
		}
	}

	return nil
}

func (db *dagBuilderHelper) fillNodeWithData(node *unixfsNode) error {
	data := db.next()
	if data == nil { // we're done!
		return nil
	}

	if len(data) > BlockSizeLimit {
		return ErrSizeLimitExceeded
	}

	node.setData(data)
	return nil
}

func (db *dagBuilderHelper) fillStreamNodeRec(node *unixfsNode, depth int) error {
	if depth < 0 {
		return errors.New("attempt to fillNode at depth < 0")
	}

	// Base case
	if depth <= 0 { // catch accidental -1's in case error above is removed.
		return db.fillNodeWithData(node)
	}

	// Store data in the link nodes to lower latency reads
	err := db.fillNodeWithData(node)
	if err != nil {
		return err
	}

	// while we have room AND we're not done
	for node.numChildren() < db.maxlinks && !db.done() {
		child := newUnixfsNode()

		if err := db.fillStreamNodeRec(child, depth-1); err != nil {
			return err
		}

		if err := node.addChild(child, db); err != nil {
			return err
		}
	}

	return nil
}

// why is intmin not in math?
func min(a, b int) int {
	if a > b {
		return a
	}
	return b
}
