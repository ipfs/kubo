package path

import (
	"fmt"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
	"path"
	"strings"
)

// Path resolution for IPFS

type Resolver struct {
	DAG *merkledag.DAGService
}

func (s *Resolver) ResolvePath(fpath string) (*merkledag.Node, error) {
	fpath = path.Clean(fpath)

	parts := strings.Split(fpath, "/")

	// skip over empty first elem
	if len(parts[0]) == 0 {
		parts = parts[1:]
	}

	// if nothing, bail.
	if len(parts) == 0 {
		return nil, fmt.Errorf("ipfs path must contain at least one component.")
	}

	// first element in the path is a b58 hash (for now)
	h, err := mh.FromB58String(parts[0])
	if err != nil {
		return nil, err
	}

	nd, err := s.DAG.Get(u.Key(h))
	if err != nil {
		return nil, err
	}

	return s.ResolveLinks(nd, parts[1:])
}

func (s *Resolver) ResolveLinks(ndd *merkledag.Node, names []string) (
	nd *merkledag.Node, err error) {

	nd = ndd // dup arg workaround

	// for each of the path components
	for _, name := range names {

		var next u.Key
		// for each of the links in nd, the current object
		for _, link := range nd.Links {
			if link.Name == name {
				next = u.Key(link.Hash)
				break
			}
		}

		if next == "" {
			h1, _ := nd.Multihash()
			h2 := h1.B58String()
			return nil, fmt.Errorf("no link named %q under %s", name, h2)
		}

		// fetch object for link and assign to nd
		nd, err = s.DAG.Get(next)
		if err != nil {
			return nd, err
		}
	}
	return
}
