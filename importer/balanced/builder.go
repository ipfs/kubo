// Package balanced provides methods to build balanced DAGs.
// In a balanced DAG, nodes are added to a single root
// until the maximum number of links is reached (with leaves
// being at depth 0). Then, a new root is created, and points to the
// old root, and incorporates a new child, which proceeds to be
// filled up (link) to more leaves. In all cases, the Data (chunks)
// is stored only at the leaves, with the rest of nodes only
// storing links to their children.
//
// In a balanced DAG, nodes fill their link capacity before
// creating new ones, thus depth only increases when the
// current tree is completely full.
//
// Balanced DAGs are generalistic DAGs in which all leaves
// are at the same distance from the root.
package balanced

import (
	"errors"

	h "github.com/ipfs/go-ipfs/importer/helpers"

	ipld "gx/ipfs/QmWi2BYBL5gJ3CiAiQchg6rn1A8iBsrWy51EYxvHVjFvLb/go-ipld-format"
)

// Layout builds a balanced DAG. Data is stored at the leaves
// and depth only increases when the tree is full, that is, when
// the root node has reached the maximum number of links.
func Layout(db *h.DagBuilderHelper) (ipld.Node, error) {
	var rootFileSize uint64
	var root *h.UnixfsNode
	var level int
	var err error
	var filledRoot ipld.Node
	for level = 0; !db.Done(); level++ {

		nroot := db.NewUnixfsNode()
		db.SetPosInfo(nroot, 0)

		// add our old root as a child of the new root.
		if root != nil { // nil if it's the first node.
			if err := nroot.AddChildNode(filledRoot, db, rootFileSize); err != nil {
				return nil, err
			}
		}

		// fill it up.
		if filledRoot, rootFileSize, err = fillNodeRec(db, *nroot, level, rootFileSize); err != nil {
			return nil, err
		}

		root = nroot

	}

	if root == nil {
		// this should only happen with an empty node, so return a leaf
		root, err = db.NewLeaf(nil)
		if err != nil {
			return nil, err
		}
	}

	var rootDagNode ipld.Node
	if level != 1 {
		rootDagNode, err = root.GetDagNode()
		if err != nil {
			return nil, err
		}
	} else {
		// The loop ended (`db.Done()`) before the first `AddChildNode` was
		// called, `root` doesn't contain the DAG, there is a single (leaf)
		// node (with data) returned in `filledRoot`, so add that.
		rootDagNode = filledRoot
	}
	out, err := db.AddDagNode(rootDagNode)
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
// it returns the input node now filled (`filledNode`), its total `fileSize`,
// and a potential error
//
// warning: **children** pinned indirectly, but input node IS NOT pinned.
func fillNodeRec(db *h.DagBuilderHelper, node h.UnixfsNode, depth int, offset uint64) (filledNode ipld.Node, fileSize uint64, err error) {
	if depth < 0 {
		return nil, 0, errors.New("attempt to fillNode at depth < 0")
	}

	// Base case
	if depth <= 0 { // catch accidental -1's in case error above is removed.
		child, err := db.GetNextDataNode()
		if err != nil {
			return nil, 0, err
		}

		node.Set(child)
		return node.GetDagNodeFileSize()
	}

	// while we have room AND we're not done
	for node.NumChildren() < db.Maxlinks() && !db.Done() {
		child := db.NewUnixfsNode()
		db.SetPosInfo(child, offset)

		filledChild, childFileSize, err := fillNodeRec(db, *child, depth-1, offset)
		if err != nil {
			return nil, 0, err
		}

		if err = node.AddChildNode(filledChild, db, childFileSize); err != nil {
			return nil, 0, err
		}
		offset += childFileSize
	}

	return node.GetDagNodeFileSize()
}
