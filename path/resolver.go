// Package path implements utilities for resolving paths within ipfs.
package path

import (
	"context"
	"errors"
	"fmt"
	"time"

	merkledag "github.com/ipfs/go-ipfs/merkledag"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"
	node "gx/ipfs/QmZx42H5khbVQhV5odp66TApShV4XCujYazcvYduZ4TroB/go-ipld-node"
)

var log = logging.Logger("path")

// Paths after a protocol must contain at least one component
var ErrNoComponents = errors.New(
	"path must contain at least one component")

// ErrNoLink is returned when a link is not found in a path
type ErrNoLink struct {
	Name string
	Node *cid.Cid
}

func (e ErrNoLink) Error() string {
	return fmt.Sprintf("no link named %q under %s", e.Name, e.Node.String())
}

// Resolver provides path resolution to IPFS
// It has a pointer to a DAGService, which is uses to resolve nodes.
type Resolver struct {
	DAG merkledag.DAGService
}

// SplitAbsPath clean up and split fpath. It extracts the first component (which
// must be a Multihash) and return it separately.
func SplitAbsPath(fpath Path) (*cid.Cid, []string, error) {

	log.Debugf("Resolve: '%s'", fpath)

	parts := fpath.Segments()
	if parts[0] == "ipfs" {
		parts = parts[1:]
	}

	// if nothing, bail.
	if len(parts) == 0 {
		return nil, nil, ErrNoComponents
	}

	c, err := cid.Decode(parts[0])
	if err != nil {
		return nil, nil, err
	}

	return c, parts[1:], nil
}

// ResolvePath fetches the node for given path. It returns the last item
// returned by ResolvePathComponents.
func (s *Resolver) ResolvePath(ctx context.Context, fpath Path) (node.Node, error) {
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
func (s *Resolver) ResolvePathComponents(ctx context.Context, fpath Path) ([]node.Node, error) {
	h, parts, err := SplitAbsPath(fpath)
	if err != nil {
		return nil, err
	}

	log.Debug("resolve dag get")
	nd, err := s.DAG.Get(ctx, h)
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
func (s *Resolver) ResolveLinks(ctx context.Context, ndd node.Node, names []string) ([]node.Node, error) {

	result := make([]node.Node, 0, len(names)+1)
	result = append(result, ndd)
	nd := ndd // dup arg workaround

	// for each of the path components
	for len(names) > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Minute)
		defer cancel()

		lnk, rest, err := nd.Resolve(names)
		if err == merkledag.ErrLinkNotFound {
			n := nd.Cid()
			return result, ErrNoLink{Name: names[0], Node: n}
		} else if err != nil {
			return result, err
		}

		nextnode, err := s.DAG.Get(ctx, lnk.Cid)
		if err != nil {
			return result, err
		}

		nd = nextnode
		names = rest
		result = append(result, nextnode)
	}
	return result, nil
}
