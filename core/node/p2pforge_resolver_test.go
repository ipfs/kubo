package node

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constants matching p2p-forge production format
const (
	// testPeerID is a valid peerID in CIDv1 base36 format as used by p2p-forge.
	// Base36 is lowercase-only, making it safe for case-insensitive DNS.
	// Corresponds to 12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN in base58btc.
	testPeerID = "k51qzi5uqu5dhnwe629wdlncpql6frppdpwnz4wtlcw816aysd5wwlk63g4wmh"

	// domainSuffix is the default p2p-forge domain used in tests.
	domainSuffix = config.DefaultDomainSuffix
)

// mockResolver implements madns.BasicResolver for testing
type mockResolver struct {
	txtRecords map[string][]string
	ipRecords  map[string][]net.IPAddr
	ipErr      error
}

func (m *mockResolver) LookupIPAddr(_ context.Context, hostname string) ([]net.IPAddr, error) {
	if m.ipErr != nil {
		return nil, m.ipErr
	}
	if m.ipRecords != nil {
		return m.ipRecords[hostname], nil
	}
	return nil, nil
}

func (m *mockResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if m.txtRecords != nil {
		return m.txtRecords[name], nil
	}
	return nil, nil
}

// newTestResolver creates a p2pForgeResolver with default suffix.
func newTestResolver(t *testing.T) *p2pForgeResolver {
	t.Helper()
	return NewP2PForgeResolver([]string{domainSuffix}, &mockResolver{})
}

// assertLookupIP verifies that hostname resolves to wantIP.
func assertLookupIP(t *testing.T, r *p2pForgeResolver, hostname, wantIP string) {
	t.Helper()
	addrs, err := r.LookupIPAddr(t.Context(), hostname)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, wantIP, addrs[0].IP.String())
}

func TestP2PForgeResolver_LookupIPAddr(t *testing.T) {
	r := newTestResolver(t)

	tests := []struct {
		name     string
		hostname string
		wantIP   string
	}{
		// IPv4
		{"ipv4/basic", "192-168-1-1." + testPeerID + "." + domainSuffix, "192.168.1.1"},
		{"ipv4/zeros", "0-0-0-0." + testPeerID + "." + domainSuffix, "0.0.0.0"},
		{"ipv4/max", "255-255-255-255." + testPeerID + "." + domainSuffix, "255.255.255.255"},
		{"ipv4/trailing dot", "10-0-0-1." + testPeerID + "." + domainSuffix + ".", "10.0.0.1"},
		{"ipv4/uppercase suffix", "192-168-1-1." + testPeerID + ".LIBP2P.DIRECT", "192.168.1.1"},
		// IPv6
		{"ipv6/full", "2001-db8-0-0-0-0-0-1." + testPeerID + "." + domainSuffix, "2001:db8::1"},
		{"ipv6/compressed", "2001-db8--1." + testPeerID + "." + domainSuffix, "2001:db8::1"},
		{"ipv6/loopback", "0--1." + testPeerID + "." + domainSuffix, "::1"},
		{"ipv6/all zeros", "0--0." + testPeerID + "." + domainSuffix, "::"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertLookupIP(t, r, tt.hostname, tt.wantIP)
		})
	}
}

func TestP2PForgeResolver_LookupIPAddr_MultipleSuffixes(t *testing.T) {
	r := NewP2PForgeResolver([]string{domainSuffix, "custom.example.com"}, &mockResolver{})

	tests := []struct {
		hostname string
		wantIP   string
	}{
		{"192-168-1-1." + testPeerID + "." + domainSuffix, "192.168.1.1"},
		{"10-0-0-1." + testPeerID + ".custom.example.com", "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			assertLookupIP(t, r, tt.hostname, tt.wantIP)
		})
	}
}

func TestP2PForgeResolver_LookupIPAddr_FallbackToNetwork(t *testing.T) {
	fallbackIP := []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}

	tests := []struct {
		name     string
		hostname string
	}{
		{"peerID only", testPeerID + "." + domainSuffix},
		{"invalid peerID", "192-168-1-1.invalid-peer-id." + domainSuffix},
		{"invalid IP encoding", "not-an-ip." + testPeerID + "." + domainSuffix},
		{"leading hyphen", "-192-168-1-1." + testPeerID + "." + domainSuffix},
		{"too many parts", "extra.192-168-1-1." + testPeerID + "." + domainSuffix},
		{"wrong suffix", "192-168-1-1." + testPeerID + ".example.com"},
	}

	// Build fallback records from test cases
	ipRecords := make(map[string][]net.IPAddr, len(tests))
	for _, tt := range tests {
		ipRecords[tt.hostname] = fallbackIP
	}
	fallback := &mockResolver{ipRecords: ipRecords}
	r := NewP2PForgeResolver([]string{domainSuffix}, fallback)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs, err := r.LookupIPAddr(t.Context(), tt.hostname)
			require.NoError(t, err)
			require.Len(t, addrs, 1, "should fallback to network")
			assert.Equal(t, "93.184.216.34", addrs[0].IP.String())
		})
	}
}

func TestP2PForgeResolver_LookupIPAddr_FallbackError(t *testing.T) {
	expectedErr := errors.New("network error")
	r := NewP2PForgeResolver([]string{domainSuffix}, &mockResolver{ipErr: expectedErr})

	// peerID-only triggers fallback, which returns error
	_, err := r.LookupIPAddr(t.Context(), testPeerID+"."+domainSuffix)
	require.ErrorIs(t, err, expectedErr)
}

func TestP2PForgeResolver_LookupTXT(t *testing.T) {
	t.Run("delegates to fallback for ACME DNS-01", func(t *testing.T) {
		acmeHost := "_acme-challenge." + testPeerID + "." + domainSuffix
		fallback := &mockResolver{
			txtRecords: map[string][]string{acmeHost: {"acme-token-value"}},
		}
		r := NewP2PForgeResolver([]string{domainSuffix}, fallback)

		records, err := r.LookupTXT(t.Context(), acmeHost)
		require.NoError(t, err)
		assert.Equal(t, []string{"acme-token-value"}, records)
	})

	t.Run("returns empty when fallback has no records", func(t *testing.T) {
		r := NewP2PForgeResolver([]string{domainSuffix}, &mockResolver{})

		records, err := r.LookupTXT(t.Context(), "anything."+domainSuffix)
		require.NoError(t, err)
		assert.Empty(t, records)
	})
}
