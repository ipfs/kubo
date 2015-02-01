package importer

import (
	"io"

	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/pin"
	ft "github.com/jbenet/go-ipfs/unixfs"
	u "github.com/jbenet/go-ipfs/util"
)

func BuildFastDagFromReader(r io.Reader, ds dag.DAGService, mp pin.ManualPinner, spl chunk.BlockSplitter) (*dag.Node, error) {
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

	root := newUnixfsNode()

	for level := 1; !db.done(); level++ {
		rotate(db, root)
		err := db.fillStreamNodeRec(root, level)
		if err != nil {
			return nil, err
		}
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

// rotate performs a recursive tree rotation down the leftmost child
// this attains a moderately balanced tree while retaining node odering
// postconditions: The given node will have a single child, and a
// correctly adjusted MultiBlock object
func rotate(db *dagBuilderHelper, node *unixfsNode) error {
	// base case
	if node.numChildren() == 0 {
		return nil
	}

	// Grab our links
	links := node.node.Links

	// Get our first child (have to reload for modification)
	leftnode, err := db.dserv.Get(u.Key(links[0].Hash))
	if err != nil {
		return err
	}

	lfsnode, err := unixfsNodeFromDagNode(leftnode)
	if err != nil {
		return err
	}

	// rotate our first child tree
	err = rotate(db, lfsnode)
	if err != nil {
		return err
	}

	// Add all our children to our left child
	for i, l := range links[1:] {
		lfsnode.node.Links = append(lfsnode.node.Links, l)
		lfsnode.ufmt.AddBlockSize(node.ufmt.GetBlocksizes()[i])
	}

	// Reset our current node, and re-add our lone child
	node.ufmt = new(ft.MultiBlock)
	node.node.Links = nil
	err = node.addChild(lfsnode, db)
	if err != nil {
		return err
	}

	return nil
}
