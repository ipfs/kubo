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
		for j := 0; j < layerRepeat && !db.Done(); j++ {
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

// TrickleAppend appends the data in `db` to the dag, using the Trickledag format
func TrickleAppend(base *dag.Node, db *h.DagBuilderHelper) (*dag.Node, error) {
	// Convert to unixfs node for working with easily
	ufsn, err := h.NewUnixfsNodeFromDag(base)
	if err != nil {
		return nil, err
	}

	// Get depth of this 'tree'
	n, j := trickleDepthInfo(base, db.Maxlinks())
	if n == 0 {
		// If direct blocks not filled...
		err := db.FillNodeLayer(ufsn)
		if err != nil {
			return nil, err
		}
		n++

		if db.Done() {
			return ufsn.GetDagNode()
		}
	}

	if len(base.Links) > db.Maxlinks() {
		// Recursive step, grab last child
		last := len(base.Links) - 1
		lastChild, err := base.Links[last].GetNode(db.GetDagServ())
		if err != nil {
			return nil, err
		}

		// Fill out last child (may not be full tree)
		nchild, err := trickleAppendRec(lastChild, db, n)
		if err != nil {
			return nil, err
		}

		// Update node link
		ufsn.RemoveChild(last)
		err = ufsn.AddChild(nchild, db)
		if err != nil {
			return nil, err
		}

		// Partially filled depth layer
		if j != 0 {
			for ; j < layerRepeat && !db.Done(); j++ {
				next := h.NewUnixfsNode()
				err := fillTrickleRec(db, next, n)
				if err != nil {
					return nil, err
				}

				err = ufsn.AddChild(next, db)
				if err != nil {
					return nil, err
				}
			}
			n++
		}

	}

	// Now, continue filling out tree like normal
	for i := n; !db.Done(); i++ {
		for j := 0; j < layerRepeat && !db.Done(); j++ {
			next := h.NewUnixfsNode()
			err := fillTrickleRec(db, next, i)
			if err != nil {
				return nil, err
			}

			err = ufsn.AddChild(next, db)
			if err != nil {
				return nil, err
			}
		}
	}

	return ufsn.GetDagNode()
}

func trickleAppendRec(base *dag.Node, db *h.DagBuilderHelper, depth int) (*h.UnixfsNode, error) {
	// Convert to unixfs node for working with easily
	ufsn, err := h.NewUnixfsNodeFromDag(base)
	if err != nil {
		return nil, err
	}

	if depth == 0 || db.Done() {
		return ufsn, nil
	}

	// Get depth of this 'tree'
	n, j := trickleDepthInfo(base, db.Maxlinks())
	if n == 0 {
		// If direct blocks not filled...
		err := db.FillNodeLayer(ufsn)
		if err != nil {
			return nil, err
		}
		n++
	}

	// If at correct depth, no need to continue
	if n == depth {
		return ufsn, nil
	}

	if len(base.Links) > db.Maxlinks() {
		// Recursive step, grab last child
		last := len(base.Links) - 1
		lastChild, err := base.Links[last].GetNode(db.GetDagServ())
		if err != nil {
			return nil, err
		}

		// Fill out last child (may not be full tree)
		nchild, err := trickleAppendRec(lastChild, db, depth-1)
		if err != nil {
			return nil, err
		}

		// Update changed child in parent node
		ufsn.RemoveChild(last)
		err = ufsn.AddChild(nchild, db)
		if err != nil {
			return nil, err
		}

		// Partially filled depth layer
		if j != 0 {
			for ; j < layerRepeat && !db.Done(); j++ {
				next := h.NewUnixfsNode()
				err := fillTrickleRec(db, next, n)
				if err != nil {
					return nil, err
				}

				err = ufsn.AddChild(next, db)
				if err != nil {
					return nil, err
				}
			}

			// Increase depth, since we have filled out this layer
			n++
		}
	}

	// Now, continue filling out tree like normal
	for i := n; i < depth && !db.Done(); i++ {
		for j := 0; j < layerRepeat && !db.Done(); j++ {
			next := h.NewUnixfsNode()
			err := fillTrickleRec(db, next, i)
			if err != nil {
				return nil, err
			}

			err = ufsn.AddChild(next, db)
			if err != nil {
				return nil, err
			}
		}
	}

	return ufsn, nil
}

func trickleDepthInfo(node *dag.Node, maxlinks int) (int, int) {
	n := len(node.Links)
	if n < maxlinks {
		return 0, 0
	}

	return ((n - maxlinks) / layerRepeat) + 1, (n - maxlinks) % layerRepeat
}
