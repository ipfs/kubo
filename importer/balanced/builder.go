package balanced

import (
	"errors"

	h "github.com/ipfs/go-ipfs/importer/helpers"
	dag "github.com/ipfs/go-ipfs/merkledag"
)

func BalancedLayout(db *h.DagBuilderHelper) (*dag.Node, error) {
	var root *h.UnixfsNode
	for level := 0; !db.Done(); level++ {

		nroot := h.NewUnixfsNode()

		// add our old root as a child of the new root.
		if root != nil { // nil if it's the first node.
			if err := nroot.AddChild(root, db); err != nil {
				return nil, err
			}
		}

		// fill it up.
		if err := fillNodeRec(db, nroot, level); err != nil {
			return nil, err
		}

		root = nroot
	}
	if root == nil {
		root = h.NewUnixfsNode()
	}

	out, err := db.Add(root)
	if err != nil {
		return nil, err
	}

	err = db.Close()
	if err != nil {
		return nil, err
	}

	return out, nil
}

// fillNodeRec will fill the given node with data from the dagBuilders input
// source down to an indirection depth as specified by 'depth'
// it returns the total dataSize of the node, and a potential error
//
// warning: **children** pinned indirectly, but input node IS NOT pinned.
func fillNodeRec(db *h.DagBuilderHelper, node *h.UnixfsNode, depth int) error {
	if depth < 0 {
		return errors.New("attempt to fillNode at depth < 0")
	}

	// Base case
	if depth <= 0 { // catch accidental -1's in case error above is removed.
		return db.FillNodeWithData(node)
	}

	// while we have room AND we're not done
	for node.NumChildren() < db.Maxlinks() && !db.Done() {
		child := h.NewUnixfsNode()

		if err := fillNodeRec(db, child, depth-1); err != nil {
			return err
		}

		if err := node.AddChild(child, db); err != nil {
			return err
		}
	}

	return nil
}
