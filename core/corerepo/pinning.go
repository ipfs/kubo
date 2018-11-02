/*
Package corerepo provides pinning and garbage collection for local
IPFS block services.

IPFS nodes will keep local copies of any object that have either been
added or requested locally.  Not all of these objects are worth
preserving forever though, so the node administrator can pin objects
they want to keep and unpin objects that they don't care about.

Garbage collection sweeps iterate through the local block store
removing objects that aren't pinned, which frees storage space for new
objects.
*/
package corerepo

import (
	"context"
	"fmt"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi/interface"

	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
)

func Pin(n *core.IpfsNode, api iface.CoreAPI, ctx context.Context, paths []string, recursive bool) ([]cid.Cid, error) {
	out := make([]cid.Cid, len(paths))

	for i, fpath := range paths {
		p, err := iface.ParsePath(fpath)
		if err != nil {
			return nil, err
		}

		dagnode, err := api.ResolveNode(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("pin: %s", err)
		}
		err = n.Pinning.Pin(ctx, dagnode, recursive)
		if err != nil {
			return nil, fmt.Errorf("pin: %s", err)
		}
		out[i] = dagnode.Cid()
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, err
	}

	return out, nil
}

func Unpin(n *core.IpfsNode, api iface.CoreAPI, ctx context.Context, paths []string, recursive bool) ([]cid.Cid, error) {
	unpinned := make([]cid.Cid, len(paths))

	for i, p := range paths {
		p, err := iface.ParsePath(p)
		if err != nil {
			return nil, err
		}

		k, err := api.ResolvePath(ctx, p)
		if err != nil {
			return nil, err
		}

		err = n.Pinning.Unpin(ctx, k.Cid(), recursive)
		if err != nil {
			return nil, err
		}
		unpinned[i] = k.Cid()
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, err
	}
	return unpinned, nil
}
