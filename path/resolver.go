// Package path implements utilities for resolving paths within ipfs.
package path

import (
	"errors"
	"fmt"
	"time"

	mh "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	logging "github.com/ipfs/go-ipfs/vendor/QmTBXYb6y2ZcJmoXVKk3pf9rzSEjbCg7tQaJW7RSuH14nv/go-log"
)

var log = logging.Logger("path")

// Paths after a protocol must contain at least one component
var ErrNoComponents = errors.New(
	"path must contain at least one component")

// ErrNoLink is returned when a link is not found in a path
type ErrNoLink struct {
	Name string
	Node mh.Multihash
}

func (e ErrNoLink) Error() string {
	return fmt.Sprintf("no link named %q under %s", e.Name, e.Node.B58String())
}

// Resolver provides path resolution to IPFS
// It has a pointer to a DAGService, which is uses to resolve nodes.
type Resolver struct {
	DAG merkledag.DAGService
}

// SplitAbsPath clean up and split fpath. It extracts the first component (which
// must be a Multihash) and return it separately.
func SplitAbsPath(fpath Path) (mh.Multihash, []string, error) {

	log.Debugf("Resolve: '%s'", fpath)

	parts := fpath.Segments()
	if parts[0] == "ipfs" {
		parts = parts[1:]
	}

	// if nothing, bail.
	if len(parts) == 0 {
		return nil, nil, ErrNoComponents
	}

	// first element in the path is a b58 hash (for now)
	h, err := mh.FromB58String(parts[0])
	if err != nil {
		log.Debug("given path element is not a base58 string.\n")
		return nil, nil, err
	}

	return h, parts[1:], nil
}

// ResolvePath fetches the node for given path. It returns the last item
// returned by ResolvePathComponents.
func (s *Resolver) ResolvePath(ctx context.Context, fpath Path) (*merkledag.Node, error) {
	// validate path
	if err := fpath.IsValid(); err != nil {
		return nil, err
	}

	nodes, err := s.ResolvePathComponents(ctx, fpath)
	if err != nil || nodes == nil {
		return nil, err
	}
	return nodes[len(nodes)-1], err
}

// ResolvePathComponents fetches the nodes for each segment of the given path.
// It uses the first path component as a hash (key) of the first node, then
// resolves all other components walking the links, with ResolveLinks.
func (s *Resolver) ResolvePathComponents(ctx context.Context, fpath Path) ([]*merkledag.Node, error) {
	h, parts, err := SplitAbsPath(fpath)
	if err != nil {
		return nil, err
	}

	log.Debug("Resolve dag get.")
	nd, err := s.DAG.Get(ctx, key.Key(h))
	if err != nil {
		return nil, err
	}

	return s.ResolveLinks(ctx, nd, parts)
}

// ResolveLinks iteratively resolves names by walking the link hierarchy.
// Every node is fetched from the DAGService, resolving the next name.
// Returns the list of nodes forming the path, starting with ndd. This list is
// guaranteed never to be empty.
//
// ResolveLinks(nd, []string{"foo", "bar", "baz"})
// would retrieve "baz" in ("bar" in ("foo" in nd.Links).Links).Links
func (s *Resolver) ResolveLinks(ctx context.Context, ndd *merkledag.Node, names []string) ([]*merkledag.Node, error) {

	result := make([]*merkledag.Node, 0, len(names)+1)
	result = append(result, ndd)
	nd := ndd // dup arg workaround

	// for each of the path components
	for _, name := range names {

		var next key.Key
		var nlink *merkledag.Link
		// for each of the links in nd, the current object
		for _, link := range nd.Links {
			if link.Name == name {
				next = key.Key(link.Hash)
				nlink = link
				break
			}
		}

		if next == "" {
			n, _ := nd.Multihash()
			return result, ErrNoLink{Name: name, Node: n}
		}

		if nlink.Node == nil {
			// fetch object for link and assign to nd
			ctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			var err error
			nd, err = s.DAG.Get(ctx, next)
			if err != nil {
				return append(result, nd), err
			}
			nlink.Node = nd
		} else {
			nd = nlink.Node
		}

		result = append(result, nlink.Node)
	}
	return result, nil
}
