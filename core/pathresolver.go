package core

import (
	"fmt"
	"strings"

	merkledag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
)

// Resolves the given path by parsing out /ipns/ entries and then going
// through the /ipfs/ entries and returning the final merkledage node.
// Effectively enables /ipns/ in CLI commands.
func Resolve(n *IpfsNode, p path.Path) (*merkledag.Node, error) {
	strpath := string(p)

	// for now, we only try to resolve ipns paths if
	// they begin with "/ipns/". Otherwise, ambiguity
	// emerges when resolving just a <hash>. Is it meant
	// to be an ipfs or an ipns resolution?

	if strings.HasPrefix(strpath, "/ipns/") {
		// if it's an ipns path, try to resolve it.
		// if we can't, we can give that error back to the user.
		seg := p.Segments()
		if len(seg) < 2 || seg[1] == "" { // just "/ipns/"
			return nil, fmt.Errorf("invalid path: %s", string(p))
		}

		ipnsPath := seg[1]
		extensions := seg[2:]
		key, err := n.Namesys.Resolve(n.Context(), ipnsPath)
		if err != nil {
			return nil, err
		}

		pathHead := make([]string, 2)
		pathHead[0] = "ipfs"
		pathHead[1] = key.Pretty()

		p = path.FromSegments(append(pathHead, extensions...)...)
		//p = path.RebasePath(path.FromSegments(extensions...), basePath)
	}

	// ok, we have an ipfs path now (or what we'll treat as one)
	return n.Resolver.ResolvePath(p)
}
