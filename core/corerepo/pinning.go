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

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
)

func Pin(n *core.IpfsNode, ctx context.Context, paths []string, recursive bool) ([]key.Key, error) {
	dagnodes, err := core.ResolveMany(ctx, n, paths)
	if err != nil {
		return nil, fmt.Errorf("pin: %s", err)
	}

	var out []key.Key
	for _, dagnode := range dagnodes {
		k, err := dagnode.Key()
		if err != nil {
			return nil, err
		}

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		err = n.Pinning.Pin(ctx, dagnode, recursive)
		if err != nil {
			return nil, fmt.Errorf("pin: %s", err)
		}
		out = append(out, k)
	}

	if err := n.Pinning.Flush(); err != nil {
		return nil, err
	}

	return out, nil
}

func Unpin(n *core.IpfsNode, ctx context.Context, paths []string, recursive bool) ([]key.Key, error) {
	dagnodes, err := core.ResolveMany(ctx, n, paths)
	if err != nil {
		return nil, err
	}

	var unpinned []key.Key
	for _, dagnode := range dagnodes {
		k, _ := dagnode.Key()

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		err := n.Pinning.Unpin(ctx, k, recursive)
		if err != nil {
			return nil, err
		}
		unpinned = append(unpinned, k)
	}

	if err := n.Pinning.Flush(); err != nil {
		return nil, err
	}
	return unpinned, nil
}
