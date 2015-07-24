package dagutils

import (
	"errors"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
)

func AddLink(ctx context.Context, ds dag.DAGService, root *dag.Node, childname string, childk key.Key) (*dag.Node, error) {
	if childname == "" {
		return nil, errors.New("cannot create link with no name!")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
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

func InsertNodeAtPath(ctx context.Context, ds dag.DAGService, root *dag.Node, path []string, toinsert key.Key, create bool) (*dag.Node, error) {
	if len(path) == 1 {
		return AddLink(ctx, ds, root, path[0], toinsert)
	}

	nd, err := root.GetLinkedNode(ctx, ds, path[0])
	if err != nil {
		// if 'create' is true, we create directories on the way down as needed
		if err == dag.ErrNotFound && create {
			nd = &dag.Node{Data: ft.FolderPBData()}
		} else {
			return nil, err
		}
	}

	ndprime, err := InsertNodeAtPath(ctx, ds, nd, path[1:], toinsert, create)
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

func RmLink(ctx context.Context, ds dag.DAGService, root *dag.Node, path []string) (*dag.Node, error) {
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

	nnode, err := RmLink(ctx, ds, nd, path[1:])
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
