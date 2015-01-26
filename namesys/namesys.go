package namesys

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

// ipnsNameSystem implements IPNS naming.
//
// Uses three Resolvers:
// (a) ipfs routing naming: SFS-like PKI names.
// (b) dns domains: resolves using links in DNS TXT records
// (c) proquints: interprets string as the raw byte data.
//
// It can only publish to: (a) ipfs routing naming.
//
type ipns struct {
	resolvers []Resolver
	publisher Publisher
}

// NewNameSystem will construct the IPFS naming system based on Routing
func NewNameSystem(r routing.IpfsRouting) NameSystem {
	return &ipns{
		resolvers: []Resolver{
			new(DNSResolver),
			new(ProquintResolver),
			NewRoutingResolver(r),
		},
		publisher: NewRoutingPublisher(r),
	}
}

// Resolve implements Resolver
func (ns *ipns) Resolve(ctx context.Context, name string) (u.Key, error) {
	for _, r := range ns.resolvers {
		if r.CanResolve(name) {
			return r.Resolve(ctx, name)
		}
	}
	return "", ErrResolveFailed
}

// CanResolve implements Resolver
func (ns *ipns) CanResolve(name string) bool {
	for _, r := range ns.resolvers {
		if r.CanResolve(name) {
			return true
		}
	}
	return false
}

// Publish implements Publisher
func (ns *ipns) Publish(ctx context.Context, name ci.PrivKey, value u.Key) error {
	return ns.publisher.Publish(ctx, name, value)
}
