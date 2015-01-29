package corerepo

import (
	"fmt"

	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/merkledag"
	path "github.com/jbenet/go-ipfs/path"
	u "github.com/jbenet/go-ipfs/util"
)

func Pin(n *core.IpfsNode, paths []string, recursive bool) ([]u.Key, error) {

	dagnodes := make([]*merkledag.Node, 0)
	for _, fpath := range paths {
		dagnode, err := n.Resolver.ResolvePath(path.Path(fpath))
		if err != nil {
			return nil, fmt.Errorf("pin: %s", err)
		}
		dagnodes = append(dagnodes, dagnode)
	}

	var out []u.Key
	for _, dagnode := range dagnodes {
		k, err := dagnode.Key()
		if err != nil {
			return nil, err
		}

		err = n.Pinning.Pin(dagnode, recursive)
		if err != nil {
			return nil, fmt.Errorf("pin: %s", err)
		}
		out = append(out, k)
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, err
	}

	return out, nil
}

func Unpin(n *core.IpfsNode, paths []string, recursive bool) ([]u.Key, error) {

	dagnodes := make([]*merkledag.Node, 0)
	for _, fpath := range paths {
		dagnode, err := n.Resolver.ResolvePath(path.Path(fpath))
		if err != nil {
			return nil, err
		}
		dagnodes = append(dagnodes, dagnode)
	}

	var unpinned []u.Key
	for _, dagnode := range dagnodes {
		k, _ := dagnode.Key()
		err := n.Pinning.Unpin(k, recursive)
		if err != nil {
			return nil, err
		}
		unpinned = append(unpinned, k)
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, err
	}
	return unpinned, nil
}
