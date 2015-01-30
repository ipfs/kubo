// package path implements utilities for resolving paths within ipfs.
package path

import (
	"fmt"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	merkledag "github.com/jbenet/go-ipfs/struct/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("path")

// ErrNoLink is returned when a link is not found in a path
type ErrNoLink struct {
	name string
	node mh.Multihash
}

func (e ErrNoLink) Error() string {
	return fmt.Sprintf("no link named %q under %s", e.name, e.node.B58String())
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
		return nil, nil, fmt.Errorf("ipfs path must contain at least one component")
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
func (s *Resolver) ResolvePath(fpath Path) (*merkledag.Node, error) {
	nodes, err := s.ResolvePathComponents(fpath)
	if err != nil || nodes == nil {
		return nil, err
	} else {
		return nodes[len(nodes)-1], err
	}
}

// ResolvePathComponents fetches the nodes for each segment of the given path.
// It uses the first path component as a hash (key) of the first node, then
// resolves all other components walking the links, with ResolveLinks.
func (s *Resolver) ResolvePathComponents(fpath Path) ([]*merkledag.Node, error) {
	h, parts, err := SplitAbsPath(fpath)
	if err != nil {
		return nil, err
	}

	log.Debug("Resolve dag get.\n")
	nd, err := s.DAG.Get(u.Key(h))
	if err != nil {
		return nil, err
	}

	return s.ResolveLinks(nd, parts)
}

// ResolveLinks iteratively resolves names by walking the link hierarchy.
// Every node is fetched from the DAGService, resolving the next name.
// Returns the list of nodes forming the path, starting with ndd. This list is
// guaranteed never to be empty.
//
// ResolveLinks(nd, []string{"foo", "bar", "baz"})
// would retrieve "baz" in ("bar" in ("foo" in nd.Links).Links).Links
func (s *Resolver) ResolveLinks(ndd *merkledag.Node, names []string) (
	result []*merkledag.Node, err error) {

	result = make([]*merkledag.Node, 0, len(names)+1)
	result = append(result, ndd)
	nd := ndd // dup arg workaround

	// for each of the path components
	for _, name := range names {

		var next u.Key
		var nlink *merkledag.Link
		// for each of the links in nd, the current object
		for _, link := range nd.Links {
			if link.Name == name {
				next = u.Key(link.Hash)
				nlink = link
				break
			}
		}

		if next == "" {
			n, _ := nd.Multihash()
			return result, ErrNoLink{name: name, node: n}
		}

		if nlink.Node == nil {
			// fetch object for link and assign to nd
			nd, err = s.DAG.Get(next)
			if err != nil {
				return append(result, nd), err
			}
			nlink.Node = nd
		} else {
			nd = nlink.Node
		}

		result = append(result, nlink.Node)
	}
	return
}
