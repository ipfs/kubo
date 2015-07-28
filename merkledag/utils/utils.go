package dagutils

import (
	"errors"
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
)

type Editor struct {
	root *dag.Node
	ds   dag.DAGService
}

func NewDagEditor(ds dag.DAGService, root *dag.Node) *Editor {
	return &Editor{
		root: root,
		ds:   ds,
	}
}

func (e *Editor) GetNode() *dag.Node {
	return e.root.Copy()
}

func (e *Editor) AddLink(ctx context.Context, childname string, childk key.Key) error {
	nd, err := addLink(ctx, e.ds, e.root, childname, childk)
	if err != nil {
		return err
	}
	e.root = nd
	return nil
}

func addLink(ctx context.Context, ds dag.DAGService, root *dag.Node, childname string, childk key.Key) (*dag.Node, error) {
	if childname == "" {
		return nil, errors.New("cannot create link with no name!")
	}

	childnd, err := ds.Get(ctx, childk)
	if err != nil {
		return nil, err
	}

	// ensure no link with that name already exists
	_ = root.RemoveNodeLink(childname) // ignore error, only option is ErrNotFound

	err = root.AddNodeLinkClean(childname, childnd)
	if err != nil {
		return nil, err
	}

	_, err = ds.Add(root)
	if err != nil {
		return nil, err
	}
	return root, nil
}

func (e *Editor) InsertNodeAtPath(ctx context.Context, path string, toinsert key.Key, create func() *dag.Node) error {
	splpath := strings.Split(path, "/")
	nd, err := insertNodeAtPath(ctx, e.ds, e.root, splpath, toinsert, create)
	if err != nil {
		return err
	}
	e.root = nd
	return nil
}

func insertNodeAtPath(ctx context.Context, ds dag.DAGService, root *dag.Node, path []string, toinsert key.Key, create func() *dag.Node) (*dag.Node, error) {
	if len(path) == 1 {
		return addLink(ctx, ds, root, path[0], toinsert)
	}

	nd, err := root.GetLinkedNode(ctx, ds, path[0])
	if err != nil {
		// if 'create' is true, we create directories on the way down as needed
		if err == dag.ErrNotFound && create != nil {
			nd = create()
		} else {
			return nil, err
		}
	}

	ndprime, err := insertNodeAtPath(ctx, ds, nd, path[1:], toinsert, create)
	if err != nil {
		return nil, err
	}

	_ = root.RemoveNodeLink(path[0])
	err = root.AddNodeLinkClean(path[0], ndprime)
	if err != nil {
		return nil, err
	}

	_, err = ds.Add(root)
	if err != nil {
		return nil, err
	}

	return root, nil
}

func (e *Editor) RmLink(ctx context.Context, path string) error {
	splpath := strings.Split(path, "/")
	nd, err := rmLink(ctx, e.ds, e.root, splpath)
	if err != nil {
		return err
	}
	e.root = nd
	return nil
}

func rmLink(ctx context.Context, ds dag.DAGService, root *dag.Node, path []string) (*dag.Node, error) {
	if len(path) == 1 {
		// base case, remove node in question
		err := root.RemoveNodeLink(path[0])
		if err != nil {
			return nil, err
		}

		_, err = ds.Add(root)
		if err != nil {
			return nil, err
		}

		return root, nil
	}

	nd, err := root.GetLinkedNode(ctx, ds, path[0])
	if err != nil {
		return nil, err
	}

	nnode, err := rmLink(ctx, ds, nd, path[1:])
	if err != nil {
		return nil, err
	}

	_ = root.RemoveNodeLink(path[0])
	err = root.AddNodeLinkClean(path[0], nnode)
	if err != nil {
		return nil, err
	}

	_, err = ds.Add(root)
	if err != nil {
		return nil, err
	}

	return root, nil
}
