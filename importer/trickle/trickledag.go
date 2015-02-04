package trickle

import (
	h "github.com/jbenet/go-ipfs/importer/helpers"
	dag "github.com/jbenet/go-ipfs/merkledag"
)

// layerRepeat specifies how many times to append a child tree of a
// given depth. Higher values increase the width of a given node, which
// improves seek speeds.
const layerRepeat = 4

func TrickleLayout(db *h.DagBuilderHelper) (*dag.Node, error) {
	root := h.NewUnixfsNode()
	err := db.FillNodeLayer(root)
	if err != nil {
		return nil, err
	}
	for level := 1; !db.Done(); level++ {
		for i := 0; i < layerRepeat && !db.Done(); i++ {
			next := h.NewUnixfsNode()
			err := fillTrickleRec(db, next, level)
			if err != nil {
				return nil, err
			}
			err = root.AddChild(next, db)
			if err != nil {
				return nil, err
			}
		}
	}

	return db.Add(root)
}

func fillTrickleRec(db *h.DagBuilderHelper, node *h.UnixfsNode, depth int) error {
	// Always do this, even in the base case
	err := db.FillNodeLayer(node)
	if err != nil {
		return err
	}

	for i := 1; i < depth && !db.Done(); i++ {
		for j := 0; j < layerRepeat; j++ {
			next := h.NewUnixfsNode()
			err := fillTrickleRec(db, next, i)
			if err != nil {
				return err
			}

			err = node.AddChild(next, db)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
