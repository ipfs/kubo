/*
Package corerepo provides pinning and garbage collection for local
IPFS block services.

IPFS nodes will keep local copies of any object that have either been
added or requested locally.  Not all of these objects are worth
preserving forever though, so the node adminstrator can pin objects
they want to keep and unpin objects that they don't care about.

Garbage collection sweeps iterate through the local block store
removing objects that aren't pinned, which frees storage space for new
objects.
*/
package corerepo

import (
	"fmt"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	u "github.com/ipfs/go-ipfs/util"
)

func Pin(n *core.IpfsNode, paths []string, recursive bool) ([]u.Key, error) {
	// TODO(cryptix): do we want a ctx as first param for (Un)Pin() as well, just like core.Resolve?
	ctx := n.Context()

	dagnodes := make([]*merkledag.Node, 0)
	for _, fpath := range paths {
		dagnode, err := core.Resolve(ctx, n, path.Path(fpath))
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

		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		err = n.Pinning.Pin(ctx, dagnode, recursive)
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
	// TODO(cryptix): do we want a ctx as first param for (Un)Pin() as well, just like core.Resolve?
	ctx := n.Context()

	dagnodes := make([]*merkledag.Node, 0)
	for _, fpath := range paths {
		dagnode, err := core.Resolve(ctx, n, path.Path(fpath))
		if err != nil {
			return nil, err
		}
		dagnodes = append(dagnodes, dagnode)
	}

	var unpinned []u.Key
	for _, dagnode := range dagnodes {
		k, _ := dagnode.Key()

		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		err := n.Pinning.Unpin(ctx, k, recursive)
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
