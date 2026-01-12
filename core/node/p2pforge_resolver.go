package node

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"

	madns "github.com/multiformats/go-multiaddr-dns"
)

// p2pForgeResolver implements madns.BasicResolver for deterministic resolution
// of p2p-forge domains (e.g., *.libp2p.direct) without network I/O for A/AAAA queries.
//
// p2p-forge encodes IP addresses in DNS hostnames:
//   - IPv4: 1-2-3-4.peerID.libp2p.direct -> 1.2.3.4
//   - IPv6: 2001-db8--1.peerID.libp2p.direct -> 2001:db8::1
//
// TXT queries are delegated to the fallback resolver. This is important for
// p2p-forge/client ACME DNS-01 challenges to work correctly, as Let's Encrypt
// needs to verify TXT records at _acme-challenge.peerID.libp2p.direct.
//
// See: https://github.com/ipshipyard/p2p-forge
type p2pForgeResolver struct {
	suffixes []string
	fallback basicResolver
}

// Compile-time check that p2pForgeResolver implements madns.BasicResolver.
var _ madns.BasicResolver = (*p2pForgeResolver)(nil)

// basicResolver is a subset of madns.BasicResolver for TXT lookups.
type basicResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// NewP2PForgeResolver creates a resolver for the given p2p-forge domain suffixes.
// Each suffix should be a bare domain like "libp2p.direct" (without leading dot).
// TXT queries are delegated to the fallback resolver for ACME compatibility.
func NewP2PForgeResolver(suffixes []string, fallback basicResolver) *p2pForgeResolver {
	normalized := make([]string, len(suffixes))
	for i, s := range suffixes {
		normalized[i] = strings.ToLower(strings.TrimSuffix(s, "."))
	}
	return &p2pForgeResolver{suffixes: normalized, fallback: fallback}
}

// LookupIPAddr parses IP addresses encoded in the hostname.
//
// Format: <encoded-ip>.<peerID>.<suffix>
//   - IPv4: 192-168-1-1.peerID.libp2p.direct -> [192.168.1.1]
//   - IPv6: 2001-db8--1.peerID.libp2p.direct -> [2001:db8::1]
//   - PeerID only: peerID.libp2p.direct -> [] (empty, no IP component)
func (r *p2pForgeResolver) LookupIPAddr(ctx context.Context, hostname string) ([]net.IPAddr, error) {
	hostname = strings.ToLower(strings.TrimSuffix(hostname, "."))

	// find matching suffix and extract subdomain
	var subdomain string
	for _, suffix := range r.suffixes {
		if sub, found := strings.CutSuffix(hostname, "."+suffix); found {
			subdomain = sub
			break
		}
	}
	if subdomain == "" {
		return nil, fmt.Errorf("hostname %q does not match any p2p-forge suffix", hostname)
	}

	// split subdomain into parts: should be [ip-prefix, peerID] or just [peerID]
	parts := strings.Split(subdomain, ".")
	if len(parts) < 2 {
		// peerID only, no IP component - return empty (NODATA equivalent)
		return nil, nil
	}
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid p2p-forge hostname format: %q", hostname)
	}

	ipPrefix := parts[0]

	// RFC 1123: hostname labels cannot start or end with hyphen
	if len(ipPrefix) == 0 || ipPrefix[0] == '-' || ipPrefix[len(ipPrefix)-1] == '-' {
		return nil, fmt.Errorf("invalid IP encoding (RFC 1123 violation): %q", ipPrefix)
	}

	// try parsing as IPv4 first: segments joined by "-" become "."
	segments := strings.Split(ipPrefix, "-")
	if len(segments) == 4 {
		ipv4Str := strings.Join(segments, ".")
		if ip, err := netip.ParseAddr(ipv4Str); err == nil && ip.Is4() {
			return []net.IPAddr{{IP: ip.AsSlice()}}, nil
		}
	}

	// try parsing as IPv6: segments joined by "-" become ":"
	ipv6Str := strings.Join(segments, ":")
	if ip, err := netip.ParseAddr(ipv6Str); err == nil && ip.Is6() {
		return []net.IPAddr{{IP: ip.AsSlice()}}, nil
	}

	return nil, fmt.Errorf("invalid IP encoding in hostname: %q", ipPrefix)
}

// LookupTXT delegates to the fallback resolver to support ACME DNS-01 challenges
// and any other TXT record lookups on p2p-forge domains.
func (r *p2pForgeResolver) LookupTXT(ctx context.Context, hostname string) ([]string, error) {
	return r.fallback.LookupTXT(ctx, hostname)
}
