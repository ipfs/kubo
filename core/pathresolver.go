package core

import (
	"errors"
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	merkledag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	infd "github.com/ipfs/go-ipfs/util/infduration"
)

// ErrNoNamesys is an explicit error for when an IPFS node doesn't
// (yet) have a name system
var ErrNoNamesys = errors.New(
	"core/resolve: no Namesys on IpfsNode - can't resolve ipns entry")

// Resolve resolves the given path by parsing out protocol-specific
// entries (e.g. /ipns/<node-key>) and then going through the /ipfs/
// entries and returning the final merkledag node.  Effectively
// enables /ipns/, /dns/, etc. in commands.
func Resolve(ctx context.Context, n *IpfsNode, p path.Path) (*merkledag.Node, error) {
	node, _, err := ResolveWithTTL(ctx, n, p)
	return node, err
}

// ResolveWithTTL is like Resolve but also returns a time-to-live value which
// indicates the maximum amount of time the result (whether a success or an
// error) may be cached.
func ResolveWithTTL(ctx context.Context, n *IpfsNode, p path.Path) (*merkledag.Node, infd.Duration, error) {
	ttl := infd.InfiniteDuration()

	if strings.HasPrefix(p.String(), "/ipns/") {
		// resolve ipns paths

		// TODO(cryptix): we sould be able to query the local cache for the path
		if n.Namesys == nil {
			return nil, infd.FiniteDuration(0), ErrNoNamesys
		}

		seg := p.Segments()

		if len(seg) < 2 || seg[1] == "" { // just "/<protocol/>" without further segments
			return nil, infd.InfiniteDuration(), path.ErrNoComponents
		}

		extensions := seg[2:]
		resolvable, err := path.FromSegments("/", seg[0], seg[1])
		if err != nil {
			return nil, infd.InfiniteDuration(), err
		}

		resPath, resTTL, err := n.Namesys.ResolveWithTTL(ctx, resolvable.String())
		ttl = resTTL
		if err != nil {
			return nil, ttl, err
		}

		segments := append(resPath.Segments(), extensions...)
		p, err = path.FromSegments("/", segments...)
		if err != nil {
			return nil, ttl, err
		}
	}

	// ok, we have an ipfs path now (or what we'll treat as one)
	node, err := n.Resolver.ResolvePath(ctx, p)
	return node, ttl, err
}
