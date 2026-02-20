package node

import (
	"context"
	"net"
	"net/netip"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	madns "github.com/multiformats/go-multiaddr-dns"
)

// p2pForgeResolver implements madns.BasicResolver for deterministic resolution
// of p2p-forge domains (e.g., *.libp2p.direct) without network I/O for A/AAAA queries.
//
// p2p-forge encodes IP addresses in DNS hostnames:
//   - IPv4: 1-2-3-4.peerID.libp2p.direct -> 1.2.3.4
//   - IPv6: 2001-db8--1.peerID.libp2p.direct -> 2001:db8::1
//
// When local parsing fails (invalid format, invalid peerID, etc.), the resolver
// falls back to network DNS. This ensures future <peerID>.libp2p.direct records
// can still resolve if the authoritative DNS adds support for them.
//
// TXT queries always delegate to the fallback resolver. This is important for
// p2p-forge/client ACME DNS-01 challenges to work correctly, as Let's Encrypt
// needs to verify TXT records at _acme-challenge.peerID.libp2p.direct.
//
// See: https://github.com/ipshipyard/p2p-forge
type p2pForgeResolver struct {
	suffixes []string
	fallback madns.BasicResolver
}

// Compile-time check that p2pForgeResolver implements madns.BasicResolver.
var _ madns.BasicResolver = (*p2pForgeResolver)(nil)

// NewP2PForgeResolver creates a resolver for the given p2p-forge domain suffixes.
// Each suffix should be a bare domain like "libp2p.direct" (without leading dot).
// When local IP parsing fails, queries fall back to the provided resolver.
// TXT queries always delegate to the fallback resolver for ACME compatibility.
func NewP2PForgeResolver(suffixes []string, fallback madns.BasicResolver) *p2pForgeResolver {
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
//
// If the hostname doesn't match the expected format (wrong suffix, invalid peerID,
// invalid IP encoding, or peerID-only), the lookup falls back to network DNS.
// This allows future DNS records like <peerID>.libp2p.direct to resolve normally.
func (r *p2pForgeResolver) LookupIPAddr(ctx context.Context, hostname string) ([]net.IPAddr, error) {
	// DNS is case-insensitive, normalize to lowercase
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
		// not a p2p-forge domain, fallback to network
		return r.fallback.LookupIPAddr(ctx, hostname)
	}

	// split subdomain into parts: should be [ip-prefix, peerID]
	parts := strings.Split(subdomain, ".")
	if len(parts) != 2 {
		// not the expected <ip>.<peerID> format, fallback to network
		return r.fallback.LookupIPAddr(ctx, hostname)
	}

	encodedIP := parts[0]
	peerIDStr := parts[1]

	// validate peerID (same validation as libp2p.direct DNS server)
	if _, err := peer.Decode(peerIDStr); err != nil {
		// invalid peerID, fallback to network
		return r.fallback.LookupIPAddr(ctx, hostname)
	}

	// RFC 1123: hostname labels cannot start or end with hyphen
	if len(encodedIP) == 0 || encodedIP[0] == '-' || encodedIP[len(encodedIP)-1] == '-' {
		// invalid hostname label, fallback to network
		return r.fallback.LookupIPAddr(ctx, hostname)
	}

	// try parsing as IPv4 first: segments joined by "-" become "."
	segments := strings.Split(encodedIP, "-")
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

	// IP parsing failed, fallback to network
	return r.fallback.LookupIPAddr(ctx, hostname)
}

// LookupTXT delegates to the fallback resolver to support ACME DNS-01 challenges
// and any other TXT record lookups on p2p-forge domains.
func (r *p2pForgeResolver) LookupTXT(ctx context.Context, hostname string) ([]string, error) {
	return r.fallback.LookupTXT(ctx, hostname)
}
