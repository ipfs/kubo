package namesys

import (
	"errors"
	"net"
	"strings"

	isd "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-is-domain"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	path "github.com/ipfs/go-ipfs/path"
	infd "github.com/ipfs/go-ipfs/util/infduration"
)

type LookupTXTFunc func(name string) (txt []string, err error)

// DNSResolver implements a Resolver on DNS domains
type DNSResolver struct {
	lookupTXT LookupTXTFunc
	// TODO: maybe some sort of caching?
	// cache would need a timeout
}

// NewDNSResolver constructs a name resolver using DNS TXT records.
func NewDNSResolver() Resolver {
	return &DNSResolver{lookupTXT: net.LookupTXT}
}

// newDNSResolver constructs a name resolver using DNS TXT records,
// returning a resolver instead of NewDNSResolver's Resolver.
func newDNSResolver() resolver {
	return &DNSResolver{lookupTXT: net.LookupTXT}
}

// Resolve implements Resolver.
func (r *DNSResolver) Resolve(ctx context.Context, name string) (path.Path, error) {
	p, _, err := r.ResolveWithTTL(ctx, name)
	return p, err
}

// ResolveN implements Resolver.
func (r *DNSResolver) ResolveN(ctx context.Context, name string, depth int) (path.Path, error) {
	p, _, err := r.ResolveNWithTTL(ctx, name, depth)
	return p, err
}

// ResolveWithTTL implements Resolver.
func (r *DNSResolver) ResolveWithTTL(ctx context.Context, name string) (path.Path, infd.Duration, error) {
	return r.ResolveNWithTTL(ctx, name, DefaultDepthLimit)
}

// ResolveNWithTTL implements Resolver.
func (r *DNSResolver) ResolveNWithTTL(ctx context.Context, name string, depth int) (path.Path, infd.Duration, error) {
	return resolve(ctx, r, name, depth, "/ipns/")
}

// resolveOnce implements resolver.
// TXT records for a given domain name should contain a b58
// encoded multihash.
func (r *DNSResolver) resolveOnce(ctx context.Context, name string) (path.Path, infd.Duration, error) {
	if !isd.IsDomain(name) {
		// IsDomain may change, do not use InfiniteDuration.
		return "", infd.FiniteDuration(UnknownTTL), errors.New("not a valid domain name")
	}

	log.Infof("DNSResolver resolving %s", name)

	// XXX: net.LookupTXT does not provide TTL information, use UnknownTTL
	// (1 minute) for now.
	ttl := infd.FiniteDuration(UnknownTTL)

	txt, err := r.lookupTXT(name)
	if err != nil {
		return "", infd.FiniteDuration(0), err
	}

	for _, t := range txt {
		p, err := parseEntry(t)
		if err == nil {
			return p, ttl, nil
		}
	}

	// We received a result but it had no entries.  This can be cached.
	return "", ttl, ErrResolveFailed
}

func parseEntry(txt string) (path.Path, error) {
	p, err := path.ParseKeyToPath(txt) // bare IPFS multihashes
	if err == nil {
		return p, nil
	}

	return tryParseDnsLink(txt)
}

func tryParseDnsLink(txt string) (path.Path, error) {
	parts := strings.SplitN(txt, "=", 2)
	if len(parts) == 2 && parts[0] == "dnslink" {
		return path.ParsePath(parts[1])
	}

	return "", errors.New("not a valid dnslink entry")
}
