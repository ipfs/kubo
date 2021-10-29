package namesys

import (
	"context"
	"errors"
	"fmt"
	"net"
	gpath "path"
	"strings"

	path "github.com/ipfs/go-path"
	opts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	dns "github.com/miekg/dns"
)

// LookupTXTFunc is a function that lookups TXT record values.
type LookupTXTFunc func(ctx context.Context, name string) (txt []string, err error)

// DNSResolver implements a Resolver on DNS domains
type DNSResolver struct {
	lookupTXT LookupTXTFunc
	// TODO: maybe some sort of caching?
	// cache would need a timeout
}

// NewDNSResolver constructs a name resolver using DNS TXT records.
func NewDNSResolver(lookup LookupTXTFunc) *DNSResolver {
	return &DNSResolver{lookupTXT: lookup}
}

// Resolve implements Resolver.
func (r *DNSResolver) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	return resolve(ctx, r, name, opts.ProcessOpts(options))
}

// ResolveAsync implements Resolver.
func (r *DNSResolver) ResolveAsync(ctx context.Context, name string, options ...opts.ResolveOpt) <-chan Result {
	return resolveAsync(ctx, r, name, opts.ProcessOpts(options))
}

type lookupRes struct {
	path  path.Path
	error error
}

// resolveOnce implements resolver.
// TXT records for a given domain name should contain a b58
// encoded multihash.
func (r *DNSResolver) resolveOnceAsync(ctx context.Context, name string, options opts.ResolveOpts) <-chan onceResult {
	var fqdn string
	out := make(chan onceResult, 1)
	segments := strings.SplitN(name, "/", 2)
	domain := segments[0]

	if _, ok := dns.IsDomainName(domain); !ok {
		out <- onceResult{err: fmt.Errorf("not a valid domain name: %s", domain)}
		close(out)
		return out
	}
	log.Debugf("DNSResolver resolving %s", domain)

	if strings.HasSuffix(domain, ".") {
		fqdn = domain
	} else {
		fqdn = domain + "."
	}

	rootChan := make(chan lookupRes, 1)
	go workDomain(ctx, r, fqdn, rootChan)

	subChan := make(chan lookupRes, 1)
	go workDomain(ctx, r, "_dnslink."+fqdn, subChan)

	appendPath := func(p path.Path) (path.Path, error) {
		if len(segments) > 1 {
			return path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[1])
		}
		return p, nil
	}

	go func() {
		defer close(out)
		var rootResErr, subResErr error
		for {
			select {
			case subRes, ok := <-subChan:
				if !ok {
					subChan = nil
					break
				}
				if subRes.error == nil {
					p, err := appendPath(subRes.path)
					emitOnceResult(ctx, out, onceResult{value: p, err: err})
					// Return without waiting for rootRes, since this result
					// (for "_dnslink."+fqdn) takes precedence
					return
				}
				subResErr = subRes.error
			case rootRes, ok := <-rootChan:
				if !ok {
					rootChan = nil
					break
				}
				if rootRes.error == nil {
					p, err := appendPath(rootRes.path)
					emitOnceResult(ctx, out, onceResult{value: p, err: err})
					// Do not return here.  Wait for subRes so that it is
					// output last if good, thereby giving subRes precedence.
				} else {
					rootResErr = rootRes.error
				}
			case <-ctx.Done():
				return
			}
			if subChan == nil && rootChan == nil {
				// If here, then both lookups are done
				//
				// If both lookups failed due to no TXT records with a
				// dnslink, then output a more specific error message
				if rootResErr == ErrResolveFailed && subResErr == ErrResolveFailed {
					// Wrap error so that it can be tested if it is a ErrResolveFailed
					err := fmt.Errorf("%w: %q is missing a DNSLink record (https://docs.ipfs.io/concepts/dnslink/)", ErrResolveFailed, gpath.Base(name))
					emitOnceResult(ctx, out, onceResult{err: err})
				}
				return
			}
		}
	}()

	return out
}

func workDomain(ctx context.Context, r *DNSResolver, name string, res chan lookupRes) {
	defer close(res)

	txt, err := r.lookupTXT(ctx, name)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok {
			// If no TXT records found, return same error as when no text
			// records contain dnslink. Otherwise, return the actual error.
			if dnsErr.IsNotFound {
				err = ErrResolveFailed
			}
		}
		// Could not look up any text records for name
		res <- lookupRes{"", err}
		return
	}

	for _, t := range txt {
		p, err := parseEntry(t)
		if err == nil {
			res <- lookupRes{p, nil}
			return
		}
	}

	// There were no TXT records with a dnslink
	res <- lookupRes{"", ErrResolveFailed}
}

func parseEntry(txt string) (path.Path, error) {
	p, err := path.ParseCidToPath(txt) // bare IPFS multihashes
	if err == nil {
		return p, nil
	}

	return tryParseDNSLink(txt)
}

func tryParseDNSLink(txt string) (path.Path, error) {
	parts := strings.SplitN(txt, "=", 2)
	if len(parts) == 2 && parts[0] == "dnslink" {
		return path.ParsePath(parts[1])
	}

	return "", errors.New("not a valid dnslink entry")
}
