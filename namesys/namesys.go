package namesys

import (
	"strings"
	"time"

	context "context"
	path "github.com/ipfs/go-ipfs/path"
	routing "gx/ipfs/QmQKEgGgYCDyk8VNY6A65FpuE4YwbspvjXHco1rdb75PVc/go-libp2p-routing"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	ci "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
)

// mpns (a multi-protocol NameSystem) implements generic IPFS naming.
//
// Uses several Resolvers:
// (a) IPFS routing naming: SFS-like PKI names.
// (b) dns domains: resolves using links in DNS TXT records
// (c) proquints: interprets string as the raw byte data.
//
// It can only publish to: (a) IPFS routing naming.
//
type mpns struct {
	resolvers  map[string]resolver
	publishers map[string]Publisher
}

// NewNameSystem will construct the IPFS naming system based on Routing
func NewNameSystem(r routing.ValueStore, ds ds.Datastore, cachesize int) NameSystem {
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

const DefaultResolverCacheTTL = time.Minute

// Resolve implements Resolver.
func (ns *mpns) Resolve(ctx context.Context, name string) (path.Path, error) {
	return ns.ResolveN(ctx, name, DefaultDepthLimit)
}

// ResolveN implements Resolver.
func (ns *mpns) ResolveN(ctx context.Context, name string, depth int) (path.Path, error) {
	if strings.HasPrefix(name, "/ipfs/") {
		return path.ParsePath(name)
	}

	if !strings.HasPrefix(name, "/") {
		return path.ParsePath("/ipfs/" + name)
	}

	return resolve(ctx, ns, name, depth, "/ipns/")
}

// resolveOnce implements resolver.
func (ns *mpns) resolveOnce(ctx context.Context, name string) (path.Path, error) {
	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}
	segments := strings.SplitN(name, "/", 4)
	if len(segments) < 3 || segments[0] != "" {
		log.Warningf("Invalid name syntax for %s", name)
		return "", ErrResolveFailed
	}

	for protocol, resolver := range ns.resolvers {
		log.Debugf("Attempting to resolve %s with %s", segments[2], protocol)
		p, err := resolver.resolveOnce(ctx, segments[2])
		if err == nil {
			if len(segments) > 3 {
				return path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[3])
			} else {
				return p, err
			}
		}
	}
	log.Warningf("No resolver found for %s", name)
	return "", ErrResolveFailed
}

// Publish implements Publisher
func (ns *mpns) Publish(ctx context.Context, name ci.PrivKey, value path.Path) error {
	return ns.publishers["/ipns/"].Publish(ctx, name, value)
}

func (ns *mpns) PublishWithEOL(ctx context.Context, name ci.PrivKey, val path.Path, eol time.Time) error {
	return ns.publishers["/ipns/"].PublishWithEOL(ctx, name, val, eol)
}
