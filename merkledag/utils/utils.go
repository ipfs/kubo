package dagutils

import (
	"errors"
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

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

func (e *Editor) GetDagService() dag.DAGService {
	return e.ds
}

func addLink(ctx context.Context, ds dag.DAGService, root *dag.Node, childname string, childnd *dag.Node) (*dag.Node, error) {
	if childname == "" {
		return nil, errors.New("cannot create link with no name!")
	}

	// ensure that the node we are adding is in the dagservice
	_, err := ds.Add(childnd)
	if err != nil {
		return nil, err
	}

	// ensure no link with that name already exists
	_ = root.RemoveNodeLink(childname) // ignore error, only option is ErrNotFound

	if err := root.AddNodeLinkClean(childname, childnd); err != nil {
		return nil, err
	}

	if _, err := ds.Add(root); err != nil {
		return nil, err
	}
	return root, nil
}

func (e *Editor) InsertNodeAtPath(ctx context.Context, path string, toinsert *dag.Node, create func() *dag.Node) error {
	splpath := strings.Split(path, "/")
	nd, err := insertNodeAtPath(ctx, e.ds, e.root, splpath, toinsert, create)
	if err != nil {
		return err
	}
	e.root = nd
	return nil
}

func insertNodeAtPath(ctx context.Context, ds dag.DAGService, root *dag.Node, path []string, toinsert *dag.Node, create func() *dag.Node) (*dag.Node, error) {
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

func (e *Editor) WriteOutputTo(ds dag.DAGService) error {
	return copyDag(e.GetNode(), e.ds, ds)
}

func copyDag(nd *dag.Node, from, to dag.DAGService) error {
	_, err := to.Add(nd)
	if err != nil {
		return err
	}

	for _, lnk := range nd.Links {
		child, err := lnk.GetNode(context.Background(), from)
		if err != nil {
			if err == dag.ErrNotFound {
				// not found means we didnt modify it, and it should
				// already be in the target datastore
				continue
			}
			return err
		}

		err = copyDag(child, from, to)
		if err != nil {
			return err
		}
	}
	return nil
}
