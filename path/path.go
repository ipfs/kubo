package path

import (
	"fmt"
	"path"
	"strings"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("path")

// Resolver provides path resolution to IPFS
// It has a pointer to a DAGService, which is uses to resolve nodes.
type Resolver struct {
	DAG merkledag.DAGService
}

// ResolvePath fetches the node for given path. It uses the first
// path component as a hash (key) of the first node, then resolves
// all other components walking the links, with ResolveLinks.
func (s *Resolver) ResolvePath(fpath string) (*merkledag.Node, error) {
	log.Debugf("Resolve: '%s'", fpath)
	fpath = path.Clean(fpath)

	parts := strings.Split(fpath, "/")

	// skip over empty first elem
	if len(parts[0]) == 0 {
		parts = parts[1:]
	}

	// if nothing, bail.
	if len(parts) == 0 {
		return nil, fmt.Errorf("ipfs path must contain at least one component")
	}

	// first element in the path is a b58 hash (for now)
	h, err := mh.FromB58String(parts[0])
	if err != nil {
		log.Debug("given path element is not a base58 string.\n")
		return nil, err
	}

	log.Debug("Resolve dag get.\n")
	nd, err := s.DAG.Get(u.Key(h))
	if err != nil {
		return nil, err
	}

	return s.ResolveLinks(nd, parts[1:])
}

// ResolveLinks iteratively resolves names by walking the link hierarchy.
// Every node is fetched from the DAGService, resolving the next name.
// Returns the last node found.
//
// ResolveLinks(nd, []string{"foo", "bar", "baz"})
// would retrieve "baz" in ("bar" in ("foo" in nd.Links).Links).Links
func (s *Resolver) ResolveLinks(ndd *merkledag.Node, names []string) (
	nd *merkledag.Node, err error) {

	nd = ndd // dup arg workaround

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
			h1, _ := nd.Multihash()
			h2 := h1.B58String()
			return nil, fmt.Errorf("no link named %q under %s", name, h2)
		}

		if nlink.Node == nil {
			// fetch object for link and assign to nd
			nd, err = s.DAG.Get(next)
			if err != nil {
				return nd, err
			}
			nlink.Node = nd
		} else {
			nd = nlink.Node
		}
	}
	return
}
