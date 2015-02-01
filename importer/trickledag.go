package importer

import (
	"io"

	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/pin"
)

// layerRepeat specifies how many times to append a child tree of a
// given depth. Higher values increase the width of a given node, which
// improves seek speeds.
const layerRepeat = 4

func BuildTrickleDagFromReader(r io.Reader, ds dag.DAGService, mp pin.ManualPinner, spl chunk.BlockSplitter) (*dag.Node, error) {
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
	err := db.fillNodeRec(root, 1)
	if err != nil {
		return nil, err
	}
	for level := 1; !db.done(); level++ {
		for i := 0; i < layerRepeat && !db.done(); i++ {
			next := newUnixfsNode()
			err := db.fillTrickleRec(next, level)
			if err != nil {
				return nil, err
			}
			err = root.addChild(next, db)
			if err != nil {
				return nil, err
			}
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

func (db *dagBuilderHelper) fillTrickleRec(node *unixfsNode, depth int) error {
	// Always do this, even in the base case
	err := db.fillNodeRec(node, 1)
	if err != nil {
		return err
	}

	for i := 1; i < depth && !db.done(); i++ {
		for j := 0; j < layerRepeat; j++ {
			next := newUnixfsNode()
			err := db.fillTrickleRec(next, i)
			if err != nil {
				return err
			}

			err = node.addChild(next, db)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
