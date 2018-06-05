// Package resolver implements utilities for resolving paths within ipfs.
package resolver

import (
	"context"
	"errors"
	"fmt"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"

	logging "gx/ipfs/QmTG23dvpBCBjqQwyDxV8CQT6jmS4PSftNr1VqHhE3MLy7/go-log"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

var log = logging.Logger("pathresolv")

// ErrNoComponents is used when Paths after a protocol
// do not contain at least one component
var ErrNoComponents = errors.New(
	"path must contain at least one component")

// ErrNoLink is returned when a link is not found in a path
type ErrNoLink struct {
	Name string
	Node *cid.Cid
}

// Error implements the Error interface for ErrNoLink with a useful
// human readable message.
func (e ErrNoLink) Error() string {
	return fmt.Sprintf("no link named %q under %s", e.Name, e.Node.String())
}

// Resolver provides path resolution to IPFS
// It has a pointer to a DAGService, which is uses to resolve nodes.
// TODO: now that this is more modular, try to unify this code with the
//       the resolvers in namesys
type Resolver struct {
	DAG ipld.NodeGetter

	ResolveOnce func(ctx context.Context, ds ipld.NodeGetter, nd ipld.Node, names []string) (*ipld.Link, []string, error)
}

// NewBasicResolver constructs a new basic resolver.
func NewBasicResolver(ds ipld.DAGService) *Resolver {
	return &Resolver{
		DAG:         ds,
		ResolveOnce: ResolveSingle,
	}
}

// ResolveToLastNode walks the given path and returns the ipld.Node
// referenced by the last element in it.
func (r *Resolver) ResolveToLastNode(ctx context.Context, fpath path.Path) (ipld.Node, []string, error) {
	c, p, err := path.SplitAbsPath(fpath)
	if err != nil {
		return nil, nil, err
	}

	nd, err := r.DAG.Get(ctx, c)
	if err != nil {
		return nil, nil, err
	}

	for len(p) > 0 {
		val, rest, err := nd.Resolve(p)
		if err != nil {
			return nil, nil, err
		}

		switch val := val.(type) {
		case *ipld.Link:
			next, err := val.GetNode(ctx, r.DAG)
			if err != nil {
				return nil, nil, err
			}
			nd = next
			p = rest
		default:
			return nd, p, nil
		}
	}

	return nd, nil, nil
}

// ResolvePath fetches the node for given path. It returns the last item
// returned by ResolvePathComponents.
func (r *Resolver) ResolvePath(ctx context.Context, fpath path.Path) (ipld.Node, error) {
	// validate path
	if err := fpath.IsValid(); err != nil {
		return nil, err
	}

	nodes, err := r.ResolvePathComponents(ctx, fpath)
	if err != nil || nodes == nil {
		return nil, err
	}
	return nodes[len(nodes)-1], err
}

// ResolveSingle simply resolves one hop of a path through a graph with no
// extra context (does not opaquely resolve through sharded nodes)
func ResolveSingle(ctx context.Context, ds ipld.NodeGetter, nd ipld.Node, names []string) (*ipld.Link, []string, error) {
	return nd.ResolveLink(names)
}

// ResolvePathComponents fetches the nodes for each segment of the given path.
// It uses the first path component as a hash (key) of the first node, then
// resolves all other components walking the links, with ResolveLinks.
func (r *Resolver) ResolvePathComponents(ctx context.Context, fpath path.Path) ([]ipld.Node, error) {
	evt := log.EventBegin(ctx, "resolvePathComponents", logging.LoggableMap{"fpath": fpath})
	defer evt.Done()

	h, parts, err := path.SplitAbsPath(fpath)
	if err != nil {
		evt.Append(logging.LoggableMap{"error": err.Error()})
		return nil, err
	}

	log.Debug("resolve dag get")
	nd, err := r.DAG.Get(ctx, h)
	if err != nil {
		evt.Append(logging.LoggableMap{"error": err.Error()})
		return nil, err
	}

	return r.ResolveLinks(ctx, nd, parts)
}

// ResolveLinks iteratively resolves names by walking the link hierarchy.
// Every node is fetched from the DAGService, resolving the next name.
// Returns the list of nodes forming the path, starting with ndd. This list is
// guaranteed never to be empty.
//
// ResolveLinks(nd, []string{"foo", "bar", "baz"})
// would retrieve "baz" in ("bar" in ("foo" in nd.Links).Links).Links
func (r *Resolver) ResolveLinks(ctx context.Context, ndd ipld.Node, names []string) ([]ipld.Node, error) {

	evt := log.EventBegin(ctx, "resolveLinks", logging.LoggableMap{"names": names})
	defer evt.Done()
	result := make([]ipld.Node, 0, len(names)+1)
	result = append(result, ndd)
	nd := ndd // dup arg workaround

	// for each of the path components
	for len(names) > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Minute)
		defer cancel()

		lnk, rest, err := r.ResolveOnce(ctx, r.DAG, nd, names)
		if err == dag.ErrLinkNotFound {
			evt.Append(logging.LoggableMap{"error": err.Error()})
			return result, ErrNoLink{Name: names[0], Node: nd.Cid()}
		} else if err != nil {
			evt.Append(logging.LoggableMap{"error": err.Error()})
			return result, err
		}

		nextnode, err := lnk.GetNode(ctx, r.DAG)
		if err != nil {
			evt.Append(logging.LoggableMap{"error": err.Error()})
			return result, err
		}

		nd = nextnode
		result = append(result, nextnode)
		names = rest
	}
	return result, nil
}
