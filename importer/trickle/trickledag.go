package trickle

import (
	"errors"
	h "github.com/jbenet/go-ipfs/importer/helpers"
	dag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
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
	n, layerProgress := trickleDepthInfo(ufsn, db.Maxlinks())
	if n == 0 {
		// If direct blocks not filled...
		err := db.FillNodeLayer(ufsn)
		if err != nil {
			return nil, err
		}

		if db.Done() {
			return ufsn.GetDagNode()
		}

		// If continuing, our depth has increased by one
		n++
	}

	// Last child in this node may not be a full tree, lets file it up
	err = appendFillLastChild(ufsn, n-1, layerProgress, db)
	if err != nil {
		return nil, err
	}

	// after appendFillLastChild, our depth is now increased by one
	if !db.Done() {
		n++
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

// appendFillLastChild will take in an incomplete trickledag node (uncomplete meaning, not full) and
// fill it out to the specified depth with blocks from the given DagBuilderHelper
func appendFillLastChild(ufsn *h.UnixfsNode, depth int, layerFill int, db *h.DagBuilderHelper) error {
	if ufsn.NumChildren() <= db.Maxlinks() {
		return nil
	}
	// Recursive step, grab last child
	last := ufsn.NumChildren() - 1
	lastChild, err := ufsn.GetChild(last, db.GetDagServ())
	if err != nil {
		return err
	}

	// Fill out last child (may not be full tree)
	nchild, err := trickleAppendRec(lastChild, db, depth-1)
	if err != nil {
		return err
	}

	// Update changed child in parent node
	ufsn.RemoveChild(last)
	err = ufsn.AddChild(nchild, db)
	if err != nil {
		return err
	}

	// Partially filled depth layer
	if layerFill != 0 {
		for ; layerFill < layerRepeat && !db.Done(); layerFill++ {
			next := h.NewUnixfsNode()
			err := fillTrickleRec(db, next, depth)
			if err != nil {
				return err
			}

			err = ufsn.AddChild(next, db)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// recursive call for TrickleAppend
func trickleAppendRec(ufsn *h.UnixfsNode, db *h.DagBuilderHelper, depth int) (*h.UnixfsNode, error) {
	if depth == 0 || db.Done() {
		return ufsn, nil
	}

	// Get depth of this 'tree'
	n, layerProgress := trickleDepthInfo(ufsn, db.Maxlinks())
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

	err := appendFillLastChild(ufsn, n, layerProgress, db)
	if err != nil {
		return nil, err
	}

	// after appendFillLastChild, our depth is now increased by one
	if !db.Done() {
		n++
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

func trickleDepthInfo(node *h.UnixfsNode, maxlinks int) (int, int) {
	n := node.NumChildren()
	if n < maxlinks {
		return 0, 0
	}

	return ((n - maxlinks) / layerRepeat) + 1, (n - maxlinks) % layerRepeat
}

// VerifyTrickleDagStructure checks that the given dag matches exactly the trickle dag datastructure
// layout
func VerifyTrickleDagStructure(nd *dag.Node, ds dag.DAGService, direct int, layerRepeat int) error {
	return verifyTDagRec(nd, -1, direct, layerRepeat, ds)
}

// Recursive call for verifying the structure of a trickledag
func verifyTDagRec(nd *dag.Node, depth, direct, layerRepeat int, ds dag.DAGService) error {
	if depth == 0 {
		// zero depth dag is raw data block
		if len(nd.Links) > 0 {
			return errors.New("expected direct block")
		}

		pbn, err := ft.FromBytes(nd.Data)
		if err != nil {
			return err
		}

		if pbn.GetType() != ft.TRaw {
			return errors.New("Expected raw block")
		}
		return nil
	}

	// Verify this is a branch node
	pbn, err := ft.FromBytes(nd.Data)
	if err != nil {
		return err
	}

	if pbn.GetType() != ft.TFile {
		return errors.New("expected file as branch node")
	}

	if len(pbn.Data) > 0 {
		return errors.New("branch node should not have data")
	}

	for i := 0; i < len(nd.Links); i++ {
		child, err := nd.Links[i].GetNode(ds)
		if err != nil {
			return err
		}

		if i < direct {
			// Direct blocks
			err := verifyTDagRec(child, 0, direct, layerRepeat, ds)
			if err != nil {
				return err
			}
		} else {
			// Recursive trickle dags
			rdepth := ((i - direct) / layerRepeat) + 1
			if rdepth >= depth && depth > 0 {
				return errors.New("Child dag was too deep!")
			}
			err := verifyTDagRec(child, rdepth, direct, layerRepeat, ds)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
