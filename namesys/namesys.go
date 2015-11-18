package namesys

import (
	"strings"
	"time"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
	routing "github.com/ipfs/go-ipfs/routing"
	infd "github.com/ipfs/go-ipfs/util/infduration"
)

// mpns (a multi-protocol NameSystem) implements generic IPFS naming.
//
// Uses several Resolvers:
// (a) ipfs routing naming: SFS-like PKI names.
// (b) dns domains: resolves using links in DNS TXT records
// (c) proquints: interprets string as the raw byte data.
//
// It can only publish to: (a) ipfs routing naming.
//
type mpns struct {
	resolvers  map[string]resolver
	publishers map[string]Publisher
}

// NewNameSystem will construct the IPFS naming system based on Routing
func NewNameSystem(r routing.IpfsRouting, ds ds.Datastore, cachesize int) NameSystem {
	return &mpns{
		resolvers: map[string]resolver{
			"dns":      newDNSResolver(),
			"proquint": new(ProquintResolver),
			"dht":      NewRoutingResolver(r, cachesize),
		},
		publishers: map[string]Publisher{
			"/ipns/": NewRoutingPublisher(r, ds),
		},
	}
}

// Resolve implements Resolver.
func (ns *mpns) Resolve(ctx context.Context, name string) (path.Path, error) {
	p, _, err := ns.ResolveWithTTL(ctx, name)
	return p, err
}

// ResolveN implements Resolver.
func (ns *mpns) ResolveN(ctx context.Context, name string, depth int) (path.Path, error) {
	p, _, err := ns.ResolveNWithTTL(ctx, name, depth)
	return p, err
}

// ResolveWithTTL implements Resolver.
func (ns *mpns) ResolveWithTTL(ctx context.Context, name string) (path.Path, infd.Duration, error) {
	return ns.ResolveNWithTTL(ctx, name, DefaultDepthLimit)
}

// ResolveNWithTTL implements Resolver.
func (ns *mpns) ResolveNWithTTL(ctx context.Context, name string, depth int) (path.Path, infd.Duration, error) {
	if strings.HasPrefix(name, "/ipfs/") || !strings.HasPrefix(name, "/") {
		// ParsePath also handles paths without a / prefix.
		path, err := path.ParsePath(name)
		return path, infd.InfiniteDuration(), err
	}

	return resolve(ctx, ns, name, depth, "/ipns/")
}

// resolveOnce implements resolver.
func (ns *mpns) resolveOnce(ctx context.Context, name string) (path.Path, infd.Duration, error) {
	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}
	segments := strings.SplitN(name, "/", 3)
	if len(segments) < 3 || segments[0] != "" {
		log.Warningf("Invalid name syntax for %s", name)
		return "", infd.InfiniteDuration(), ErrResolveFailed
	}

	// Start with a long TTL.  Many errors will lower it to or near 0 in
	// the loop.
	errTTL := infd.InfiniteDuration()
	for protocol, resolver := range ns.resolvers {
		log.Debugf("Attempting to resolve %s with %s", name, protocol)
		p, resTTL, err := resolver.resolveOnce(ctx, segments[2])
		if err == nil {
			return p, resTTL, err
		}
		// Use the lowest TTL reported for errors.
		errTTL = infd.Min(errTTL, resTTL)
	}
	log.Warningf("No resolver found for %s", name)
	return "", errTTL, ErrResolveFailed
}

// Publish implements Publisher
func (ns *mpns) Publish(ctx context.Context, name ci.PrivKey, value path.Path) error {
	return ns.publishers["/ipns/"].Publish(ctx, name, value)
}

func (ns *mpns) PublishWithEOL(ctx context.Context, name ci.PrivKey, val path.Path, eol time.Time) error {
	return ns.publishers["/ipns/"].PublishWithEOL(ctx, name, val, eol)
}
