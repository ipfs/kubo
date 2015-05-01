package core

import (
	"errors"
	"fmt"
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	merkledag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
)

const maxLinks = 32

// errors returned by Resolve function
var (
	ErrTooManyLinks = errors.New("core/resolve: exceeded maximum number of links in ipns entry")
	ErrNoNamesys    = errors.New("core/resolve: no Namesys on IpfsNode - can't resolve ipns entry")
)

// Resolve resolves the given path by parsing out /ipns/ entries and then going
// through the /ipfs/ entries and returning the final merkledage node.
// Effectively enables /ipns/ in CLI commands.
func Resolve(ctx context.Context, n *IpfsNode, p path.Path) (*merkledag.Node, error) {
	r := resolver{ctx, n, p}
	return r.resolveRecurse(0)
}

type resolver struct {
	ctx context.Context
	n   *IpfsNode
	p   path.Path
}

func (r *resolver) resolveRecurse(depth int) (*merkledag.Node, error) {
	if depth >= maxLinks {
		return nil, ErrTooManyLinks
	}
	// for now, we only try to resolve ipns paths if
	// they begin with "/ipns/". Otherwise, ambiguity
	// emerges when resolving just a <hash>. Is it meant
	// to be an ipfs or an ipns resolution?

	if strings.HasPrefix(r.p.String(), "/ipns/") {
		// TODO(cryptix): we sould be able to query the local cache for the path
		if r.n.Namesys == nil {
			return nil, ErrNoNamesys
		}
		// if it's an ipns path, try to resolve it.
		// if we can't, we can give that error back to the user.
		seg := r.p.Segments()
		if len(seg) < 2 || seg[1] == "" { // just "/ipns/"
			return nil, fmt.Errorf("invalid path: %s", string(r.p))
		}

		ipnsPath := seg[1]
		extensions := seg[2:]
		respath, err := r.n.Namesys.Resolve(r.ctx, ipnsPath)
		if err != nil {
			return nil, err
		}

		segments := append(respath.Segments(), extensions...)
		r.p, err = path.FromSegments(segments...)
		if err != nil {
			return nil, err
		}
		return r.resolveRecurse(depth + 1)
	}

	// ok, we have an ipfs path now (or what we'll treat as one)
	return r.n.Resolver.ResolvePath(r.ctx, r.p)
}
