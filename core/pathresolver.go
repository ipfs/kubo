package core

import (
	"context"
	"errors"
	"strings"

	namesys "github.com/ipfs/go-ipfs/namesys"
	path "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path"
	resolver "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path/resolver"

	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
)

// ErrNoNamesys is an explicit error for when an IPFS node doesn't
// (yet) have a name system
var ErrNoNamesys = errors.New(
	"core/resolve: no Namesys on IpfsNode - can't resolve ipns entry")

// ResolveIPNS resolves /ipns paths
func ResolveIPNS(ctx context.Context, nsys namesys.NameSystem, p path.Path) (path.Path, error) {
	if strings.HasPrefix(p.String(), "/ipns/") {
		evt := log.EventBegin(ctx, "resolveIpnsPath")
		defer evt.Done()
		// resolve ipns paths

		// TODO(cryptix): we should be able to query the local cache for the path
		if nsys == nil {
			evt.Append(logging.LoggableMap{"error": ErrNoNamesys.Error()})
			return "", ErrNoNamesys
		}

		seg := p.Segments()

		if len(seg) < 2 || seg[1] == "" { // just "/<protocol/>" without further segments
			evt.Append(logging.LoggableMap{"error": path.ErrNoComponents.Error()})
			return "", path.ErrNoComponents
		}

		extensions := seg[2:]
		resolvable, err := path.FromSegments("/", seg[0], seg[1])
		if err != nil {
			evt.Append(logging.LoggableMap{"error": err.Error()})
			return "", err
		}

		respath, err := nsys.Resolve(ctx, resolvable.String())
		if err != nil {
			evt.Append(logging.LoggableMap{"error": err.Error()})
			return "", err
		}

		segments := append(respath.Segments(), extensions...)
		p, err = path.FromSegments("/", segments...)
		if err != nil {
			evt.Append(logging.LoggableMap{"error": err.Error()})
			return "", err
		}
	}
	return p, nil
}

// Resolve resolves the given path by parsing out protocol-specific
// entries (e.g. /ipns/<node-key>) and then going through the /ipfs/
// entries and returning the final node.
func Resolve(ctx context.Context, nsys namesys.NameSystem, r *resolver.Resolver, p path.Path) (ipld.Node, error) {
	p, err := ResolveIPNS(ctx, nsys, p)
	if err != nil {
		return nil, err
	}

	// ok, we have an IPFS path now (or what we'll treat as one)
	return r.ResolvePath(ctx, p)
}

// ResolveToCid resolves a path to a cid.
//
// It first checks if the path is already in the form of just a cid (<cid> or
// /ipfs/<cid>) and returns immediately if so. Otherwise, it falls back onto
// Resolve to perform resolution of the dagnode being referenced.
func ResolveToCid(ctx context.Context, nsys namesys.NameSystem, r *resolver.Resolver, p path.Path) (*cid.Cid, error) {

	// If the path is simply a cid, parse and return it. Parsed paths are already
	// normalized (read: prepended with /ipfs/ if needed), so segment[1] should
	// always be the key.
	if p.IsJustAKey() {
		return cid.Decode(p.Segments()[1])
	}

	// Fall back onto regular dagnode resolution. Retrieve the second-to-last
	// segment of the path and resolve its link to the last segment.
	head, tail, err := p.PopLastSegment()
	if err != nil {
		return nil, err
	}
	dagnode, err := Resolve(ctx, nsys, r, head)
	if err != nil {
		return nil, err
	}

	// Extract and return the cid of the link to the target dag node.
	link, _, err := dagnode.ResolveLink([]string{tail})
	if err != nil {
		return nil, err
	}

	return link.Cid, nil
}
