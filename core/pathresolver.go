package core

import (
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	merkledag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
)

// Resolve resolves the given path by parsing out protocol-specific
// entries (e.g. /ipns/<node-key>) and then going through the /ipfs/
// entries and returning the final merkledag node.  Effectively
// enables /ipns/, /dns/, etc. in commands.
func Resolve(ctx context.Context, n *IpfsNode, p path.Path) (*merkledag.Node, error) {
	if strings.HasPrefix(p.String(), "/ipns/") {
		// resolve ipns paths

		if n.Namesys == nil && !n.OnlineMode() {
			if err := n.SetupOfflineRouting(); err != nil {
				return nil, err
			}
		}

		seg := p.Segments()

		if len(seg) < 2 || seg[1] == "" { // just "/<protocol/>" without further segments
			return nil, path.ErrNoComponents
		}

		extensions := seg[2:]
		resolvable, err := path.FromSegments("/", seg[0], seg[1])
		if err != nil {
			return nil, err
		}

		respath, err := n.Namesys.Resolve(ctx, resolvable.String())
		if err != nil {
			return nil, err
		}

		segments := append(respath.Segments(), extensions...)
		p, err = path.FromSegments("/", segments...)
		if err != nil {
			return nil, err
		}
	}

	// ok, we have an ipfs path now (or what we'll treat as one)
	return n.Resolver.ResolvePath(ctx, p)
}
