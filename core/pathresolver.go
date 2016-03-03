package core

import (
	"errors"
	"strings"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
)

// ErrNoNamesys is an explicit error for when an IPFS node doesn't
// (yet) have a name system
var ErrNoNamesys = errors.New(
	"core/resolve: no Namesys on IpfsNode - can't resolve ipns entry")

// Resolve resolves the given path by parsing out protocol-specific
// entries (e.g. /ipns/<node-key>) and then going through the /ipfs/
// entries and returning the final merkledag node.
func Resolve(ctx context.Context, n *IpfsNode, p path.Path) (*merkledag.Node, error) {
	if strings.HasPrefix(p.String(), "/ipns/") {
		// resolve ipns paths

		// TODO(cryptix): we sould be able to query the local cache for the path
		if n.Namesys == nil {
			return nil, ErrNoNamesys
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

// ResolveToKey resolves a path to a key.
//
// It first checks if the path is already in the form of just a key (<key> or
// /ipfs/<key>) and returns immediately if so. Otherwise, it falls back onto
// Resolve to perform resolution of the dagnode being referenced.
func ResolveToKey(ctx context.Context, n *IpfsNode, p path.Path) (key.Key, error) {

	// If the path is simply a key, parse and return it. Parsed paths are already
	// normalized (read: prepended with /ipfs/ if needed), so segment[1] should
	// always be the key.
	if p.IsJustAKey() {
		return key.B58KeyDecode(p.Segments()[1]), nil
	}

	// Fall back onto regular dagnode resolution. Retrieve the second-to-last
	// segment of the path and resolve its link to the last segment.
	head, tail, err := p.PopLastSegment()
	if err != nil {
		return key.Key(""), err
	}
	dagnode, err := Resolve(ctx, n, head)
	if err != nil {
		return key.Key(""), err
	}

	// Extract and return the key of the link to the target dag node.
	link, err := dagnode.GetNodeLink(tail)
	if err != nil {
		return key.Key(""), err
	}

	return key.Key(link.Hash), nil
}
