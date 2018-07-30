// Package trickle allows to build trickle DAGs.
// In this type of DAG, non-leave nodes are first filled
// with data leaves, and then incorporate "layers" of subtrees
// as additional links.
//
// Each layer is a trickle sub-tree and is limited by an increasing
// maximum depth. Thus, the nodes first layer
// can only hold leaves (depth 1) but subsequent layers can grow deeper.
// By default, this module places 4 nodes per layer (that is, 4 subtrees
// of the same maximum depth before increasing it).
//
// Trickle DAGs are very good for sequentially reading data, as the
// first data leaves are directly reachable from the root and those
// coming next are always nearby. They are
// suited for things like streaming applications.
package trickle

import (
	"context"
	"errors"
	"fmt"

	dag "gx/ipfs/QmRy4Qk9hbgFX9NGJRm8rBThrA8PZhNCitMgeRYyZ67s59/go-merkledag"
	ft "gx/ipfs/QmSaz8Qg77gGqvDvLKeSAY7ivDEnramSWF6T7TcRwFpHtP/go-unixfs"
	h "gx/ipfs/QmSaz8Qg77gGqvDvLKeSAY7ivDEnramSWF6T7TcRwFpHtP/go-unixfs/importer/helpers"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
)

// layerRepeat specifies how many times to append a child tree of a
// given depth. Higher values increase the width of a given node, which
// improves seek speeds.
const layerRepeat = 4

// Layout builds a new DAG with the trickle format using the provided
// DagBuilderHelper. See the module's description for a more detailed
// explanation.
func Layout(db *h.DagBuilderHelper) (ipld.Node, error) {
	root := db.NewUnixfsNode()
	if err := fillTrickleRec(db, root, -1); err != nil {
		return nil, err
	}

	out, err := db.Add(root)
	if err != nil {
		return nil, err
	}

	if err := db.Close(); err != nil {
		return nil, err
	}

	return out, nil
}

// fillTrickleRec creates a trickle (sub-)tree with an optional maximum specified depth
// in the case maxDepth is greater than zero, or with unlimited depth otherwise
// (where the DAG builder will signal the end of data to end the function).
func fillTrickleRec(db *h.DagBuilderHelper, node *h.UnixfsNode, maxDepth int) error {
	// Always do this, even in the base case
	if err := db.FillNodeLayer(node); err != nil {
		return err
	}

	for depth := 1; ; depth++ {
		// Apply depth limit only if the parameter is set (> 0).
		if maxDepth > 0 && depth == maxDepth {
			return nil
		}
		for layer := 0; layer < layerRepeat; layer++ {
			if db.Done() {
				return nil
			}

			nextChild := db.NewUnixfsNode()
			if err := fillTrickleRec(db, nextChild, depth); err != nil {
				return err
			}

			if err := node.AddChild(nextChild, db); err != nil {
				return err
			}
		}
	}
}

// Append appends the data in `db` to the dag, using the Trickledag format
func Append(ctx context.Context, basen ipld.Node, db *h.DagBuilderHelper) (out ipld.Node, errOut error) {
	base, ok := basen.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	defer func() {
		if errOut == nil {
			if err := db.Close(); err != nil {
				errOut = err
			}
		}
	}()

	// Convert to unixfs node for working with easily
	ufsn, err := h.NewUnixfsNodeFromDag(base)
	if err != nil {
		return nil, err
	}

	// Get depth of this 'tree'
	n, layerProgress := trickleDepthInfo(ufsn, db.Maxlinks())
	if n == 0 {
		// If direct blocks not filled...
		if err := db.FillNodeLayer(ufsn); err != nil {
			return nil, err
		}

		if db.Done() {
			return ufsn.GetDagNode()
		}

		// If continuing, our depth has increased by one
		n++
	}

	// Last child in this node may not be a full tree, lets file it up
	if err := appendFillLastChild(ctx, ufsn, n-1, layerProgress, db); err != nil {
		return nil, err
	}

	// after appendFillLastChild, our depth is now increased by one
	if !db.Done() {
		n++
	}

	// Now, continue filling out tree like normal
	for i := n; !db.Done(); i++ {
		for j := 0; j < layerRepeat && !db.Done(); j++ {
			next := db.NewUnixfsNode()
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
func appendFillLastChild(ctx context.Context, ufsn *h.UnixfsNode, depth int, layerFill int, db *h.DagBuilderHelper) error {
	if ufsn.NumChildren() <= db.Maxlinks() {
		return nil
	}
	// Recursive step, grab last child
	last := ufsn.NumChildren() - 1
	lastChild, err := ufsn.GetChild(ctx, last, db.GetDagServ())
	if err != nil {
		return err
	}

	// Fill out last child (may not be full tree)
	nchild, err := appendRec(ctx, lastChild, db, depth-1)
	if err != nil {
		return err
	}

	// Update changed child in parent node
	ufsn.RemoveChild(last, db)
	err = ufsn.AddChild(nchild, db)
	if err != nil {
		return err
	}

	// Partially filled depth layer
	if layerFill != 0 {
		for ; layerFill < layerRepeat && !db.Done(); layerFill++ {
			next := db.NewUnixfsNode()
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

// recursive call for Append
func appendRec(ctx context.Context, ufsn *h.UnixfsNode, db *h.DagBuilderHelper, depth int) (*h.UnixfsNode, error) {
	if depth == 0 || db.Done() {
		return ufsn, nil
	}

	// Get depth of this 'tree'
	n, layerProgress := trickleDepthInfo(ufsn, db.Maxlinks())
	if n == 0 {
		// If direct blocks not filled...
		if err := db.FillNodeLayer(ufsn); err != nil {
			return nil, err
		}
		n++
	}

	// If at correct depth, no need to continue
	if n == depth {
		return ufsn, nil
	}

	if err := appendFillLastChild(ctx, ufsn, n, layerProgress, db); err != nil {
		return nil, err
	}

	// after appendFillLastChild, our depth is now increased by one
	if !db.Done() {
		n++
	}

	// Now, continue filling out tree like normal
	for i := n; i < depth && !db.Done(); i++ {
		for j := 0; j < layerRepeat && !db.Done(); j++ {
			next := db.NewUnixfsNode()
			if err := fillTrickleRec(db, next, i); err != nil {
				return nil, err
			}

			if err := ufsn.AddChild(next, db); err != nil {
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

// VerifyParams is used by VerifyTrickleDagStructure
type VerifyParams struct {
	Getter      ipld.NodeGetter
	Direct      int
	LayerRepeat int
	Prefix      *cid.Prefix
	RawLeaves   bool
}

// VerifyTrickleDagStructure checks that the given dag matches exactly the trickle dag datastructure
// layout
func VerifyTrickleDagStructure(nd ipld.Node, p VerifyParams) error {
	return verifyTDagRec(nd, -1, p)
}

// Recursive call for verifying the structure of a trickledag
func verifyTDagRec(n ipld.Node, depth int, p VerifyParams) error {
	codec := cid.DagProtobuf
	if depth == 0 {
		if len(n.Links()) > 0 {
			return errors.New("expected direct block")
		}
		// zero depth dag is raw data block
		switch nd := n.(type) {
		case *dag.ProtoNode:
			pbn, err := ft.FromBytes(nd.Data())
			if err != nil {
				return err
			}

			if pbn.GetType() != ft.TRaw {
				return errors.New("expected raw block")
			}

			if p.RawLeaves {
				return errors.New("expected raw leaf, got a protobuf node")
			}
		case *dag.RawNode:
			if !p.RawLeaves {
				return errors.New("expected protobuf node as leaf")
			}
			codec = cid.Raw
		default:
			return errors.New("expected ProtoNode or RawNode")
		}
	}

	// verify prefix
	if p.Prefix != nil {
		prefix := n.Cid().Prefix()
		expect := *p.Prefix // make a copy
		expect.Codec = uint64(codec)
		if codec == cid.Raw && expect.Version == 0 {
			expect.Version = 1
		}
		if expect.MhLength == -1 {
			expect.MhLength = prefix.MhLength
		}
		if prefix != expect {
			return fmt.Errorf("unexpected cid prefix: expected: %v; got %v", expect, prefix)
		}
	}

	if depth == 0 {
		return nil
	}

	nd, ok := n.(*dag.ProtoNode)
	if !ok {
		return errors.New("expected ProtoNode")
	}

	// Verify this is a branch node
	pbn, err := ft.FromBytes(nd.Data())
	if err != nil {
		return err
	}

	if pbn.GetType() != ft.TFile {
		return fmt.Errorf("expected file as branch node, got: %s", pbn.GetType())
	}

	if len(pbn.Data) > 0 {
		return errors.New("branch node should not have data")
	}

	for i := 0; i < len(nd.Links()); i++ {
		child, err := nd.Links()[i].GetNode(context.TODO(), p.Getter)
		if err != nil {
			return err
		}

		if i < p.Direct {
			// Direct blocks
			err := verifyTDagRec(child, 0, p)
			if err != nil {
				return err
			}
		} else {
			// Recursive trickle dags
			rdepth := ((i - p.Direct) / p.LayerRepeat) + 1
			if rdepth >= depth && depth > 0 {
				return errors.New("child dag was too deep")
			}
			err := verifyTDagRec(child, rdepth, p)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
