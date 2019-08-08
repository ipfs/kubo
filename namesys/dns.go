package namesys

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	config "github.com/ipfs/go-ipfs-config"
	path "github.com/ipfs/go-path"
	opts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	isd "github.com/jbenet/go-is-domain"
	ma "github.com/multiformats/go-multiaddr"
)

// LookupTXTFunc is the interface for the lookupTXT property of DNSResolver
type LookupTXTFunc func(name string) (txt []string, err error)

// DNSResolver implements a Resolver on DNS domains
type DNSResolver struct {
	lookupTXT LookupTXTFunc
}

// NewDNSResolver constructs a name resolver using DNS TXT records.
func NewDNSResolver(cfg *config.Config) (*DNSResolver, error) {
	// Check if we're using a custom DNS server
	if len(cfg.DNS.Resolver) > 0 {
		// Custom DNS resolver
		dns := &customDNS{}

		// Parse the multi-address
		resolverMaddr, err := ma.NewMultiaddr(cfg.DNS.Resolver)
		if err != nil {
			return nil, fmt.Errorf("Invalid DNS Resolver address: %q (err: %s)", cfg.DNS.Resolver, err)
		}
		proto := resolverMaddr.Protocols()

		// Need to have at least 1 component
		if len(proto) < 1 {
			return nil, fmt.Errorf("Invalid DNS Resolver address: %q (Not enough components)", cfg.DNS.Resolver)
		}

		/*for _, proto := range resolverMaddr.Protocols() {
			// Address
			if proto.Code == ma.P_IP4 || proto.Code == ma.P_IP6 {
				dns.Address =
			}
		}*/
		/*ma.ForEach(resolverMaddr, func(c ma.Component) bool {
			for _, proto := range c.Protocols() {
				val, err := c.ValueForProtocol(proto.Code)
				if err != nil {
					return false
				}
				fmt.Println(proto, val)
			}
			return true
		})*/

		// First component should be the IP or DNS
		if proto[0].Code == ma.P_IP4 || proto[0].Code == ma.P_IP6 {
			val, err := resolverMaddr.ValueForProtocol(proto[0].Code)
			if err != nil {
				return nil, fmt.Errorf("Invalid DNS Resolver address: %q (Invalid value for protocol 0)", cfg.DNS.Resolver)
			}
			dns.Address = val
		} else {
			return nil, fmt.Errorf("Invalid DNS Resolver address: %q (Invalid type for protocol 0)", cfg.DNS.Resolver)
		}

		// Second component is optional and it's the port
		// If it's UDP, we stop here, as we're using normal DNS
		// If it's TCP, we will need more info
		if len(proto) > 1 && proto[1].Code == ma.P_UDP {
			val, err := resolverMaddr.ValueForProtocol(proto[1].Code)
			if err != nil {
				return nil, fmt.Errorf("Invalid DNS Resolver address: %q (Invalid value for protocol 1)", cfg.DNS.Resolver)
			}
			dns.Protocol = "udp"
			if len(val) > 0 && val != "0" {
				n, err := strconv.ParseUint(val, 10, 32)
				if err != nil {
					return nil, err
				}
				dns.Port = uint(n)
			}
		} else if len(proto) > 2 && proto[1].Code == ma.P_TCP {
			// Require at least 3 components here because we need to know if it's using tls or https
			val, err := resolverMaddr.ValueForProtocol(proto[1].Code)
			if err != nil {
				return nil, fmt.Errorf("Invalid DNS Resolver address: %q (Invalid value for protocol 1)", cfg.DNS.Resolver)
			}
			if len(val) > 0 && val != "0" {
				n, err := strconv.ParseUint(val, 10, 32)
				if err != nil {
					return nil, err
				}
				dns.Port = uint(n)
			}

			// Need the protocol: "tls" for DNS-over-TLS or "https" for DNS-over-HTTPS
			if proto[2].Code == ma.P_HTTPS {
				// TODO: Get the host, not yet supported
				dns.Protocol = "dns-over-https"

				// This is pending https://github.com/multiformats/multicodec/pull/145 and the addition of the TLS protocol insiode https://github.com/multiformats/go-multiaddr/
				/*} else if proto[2].Code == ma.P_TLS {
				dns.Protocol = "dns-over-tls" */
			} else {
				return nil, fmt.Errorf("Invalid DNS Resolver address: %q (Invalid format)", cfg.DNS.Resolver)
			}
		} else if len(proto) > 1 {
			return nil, fmt.Errorf("Invalid DNS Resolver address: %q (Invalid format)", cfg.DNS.Resolver)
		}

		// Return the resolver
		return &DNSResolver{lookupTXT: dns.LookupTXT}, nil
	}

	// Use the system's built-in resolver
	return &DNSResolver{lookupTXT: net.LookupTXT}, nil
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

	if !isd.IsDomain(domain) {
		out <- onceResult{err: errors.New("not a valid domain name")}
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
	go workDomain(r, fqdn, rootChan)

	subChan := make(chan lookupRes, 1)
	go workDomain(r, "_dnslink."+fqdn, subChan)

	appendPath := func(p path.Path) (path.Path, error) {
		if len(segments) > 1 {
			return path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[1])
		}
		return p, nil
	}

	go func() {
		defer close(out)
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
					return
				}
			case rootRes, ok := <-rootChan:
				if !ok {
					rootChan = nil
					break
				}
				if rootRes.error == nil {
					p, err := appendPath(rootRes.path)
					emitOnceResult(ctx, out, onceResult{value: p, err: err})
				}
			case <-ctx.Done():
				return
			}
			if subChan == nil && rootChan == nil {
				return
			}
		}
	}()

	return out
}

func workDomain(r *DNSResolver, name string, res chan lookupRes) {
	defer close(res)

	txt, err := r.lookupTXT(name)
	if err != nil {
		// Error is != nil
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
	res <- lookupRes{"", ErrResolveFailed}
}

func parseEntry(txt string) (path.Path, error) {
	p, err := path.ParseCidToPath(txt) // bare IPFS multihashes
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
