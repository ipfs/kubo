package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTXTResolver is a mock fallback resolver for testing
type mockTXTResolver struct {
	records map[string][]string
}

func (m *mockTXTResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if m.records != nil {
		return m.records[name], nil
	}
	return nil, nil
}

func TestP2PForgeResolver_LookupIPAddr_IPv4(t *testing.T) {
	r := NewP2PForgeResolver([]string{"libp2p.direct"}, &mockTXTResolver{})

	tests := []struct {
		name     string
		hostname string
		wantIP   string
	}{
		{"basic", "192-168-1-1.12D3KooWPeerID.libp2p.direct", "192.168.1.1"},
		{"zeros", "0-0-0-0.peerID.libp2p.direct", "0.0.0.0"},
		{"max", "255-255-255-255.peerID.libp2p.direct", "255.255.255.255"},
		{"with trailing dot", "10-0-0-1.peerID.libp2p.direct.", "10.0.0.1"},
		{"uppercase", "192-168-1-1.PeerID.LIBP2P.DIRECT", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs, err := r.LookupIPAddr(t.Context(), tt.hostname)
			require.NoError(t, err)
			require.Len(t, addrs, 1)
			assert.Equal(t, tt.wantIP, addrs[0].IP.String())
		})
	}
}

func TestP2PForgeResolver_LookupIPAddr_IPv6(t *testing.T) {
	r := NewP2PForgeResolver([]string{"libp2p.direct"}, &mockTXTResolver{})

	tests := []struct {
		name     string
		hostname string
		wantIP   string
	}{
		{"full", "2001-db8-0-0-0-0-0-1.peerID.libp2p.direct", "2001:db8::1"},
		{"compressed", "2001-db8--1.peerID.libp2p.direct", "2001:db8::1"},
		{"loopback", "0--1.peerID.libp2p.direct", "::1"},
		{"all zeros", "0--0.peerID.libp2p.direct", "::"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs, err := r.LookupIPAddr(t.Context(), tt.hostname)
			require.NoError(t, err)
			require.Len(t, addrs, 1)
			assert.Equal(t, tt.wantIP, addrs[0].IP.String())
		})
	}
}

func TestP2PForgeResolver_LookupIPAddr_MultipleSuffixes(t *testing.T) {
	r := NewP2PForgeResolver([]string{"libp2p.direct", "custom.example.com"}, &mockTXTResolver{})

	tests := []struct {
		hostname string
		wantIP   string
	}{
		{"192-168-1-1.peerID.libp2p.direct", "192.168.1.1"},
		{"10-0-0-1.peerID.custom.example.com", "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			addrs, err := r.LookupIPAddr(t.Context(), tt.hostname)
			require.NoError(t, err)
			require.Len(t, addrs, 1)
			assert.Equal(t, tt.wantIP, addrs[0].IP.String())
		})
	}
}

func TestP2PForgeResolver_LookupIPAddr_PeerIDOnly(t *testing.T) {
	r := NewP2PForgeResolver([]string{"libp2p.direct"}, &mockTXTResolver{})

	// peerID-only should return empty (NODATA equivalent)
	addrs, err := r.LookupIPAddr(t.Context(), "12D3KooWPeerID.libp2p.direct")
	require.NoError(t, err)
	assert.Empty(t, addrs)
}

func TestP2PForgeResolver_LookupIPAddr_Errors(t *testing.T) {
	r := NewP2PForgeResolver([]string{"libp2p.direct"}, &mockTXTResolver{})

	tests := []struct {
		name     string
		hostname string
		wantErr  string
	}{
		{"wrong suffix", "192-168-1-1.peerID.example.com", "does not match any p2p-forge suffix"},
		{"invalid IP", "not-an-ip.peerID.libp2p.direct", "invalid IP encoding"},
		{"leading hyphen", "-192-168-1-1.peerID.libp2p.direct", "RFC 1123 violation"},
		{"trailing hyphen", "192-168-1-1-.peerID.libp2p.direct", "RFC 1123 violation"},
		{"too many parts", "extra.192-168-1-1.peerID.libp2p.direct", "invalid p2p-forge hostname format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.LookupIPAddr(t.Context(), tt.hostname)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestP2PForgeResolver_LookupTXT_Delegates(t *testing.T) {
	// TXT lookups should delegate to fallback resolver (for ACME DNS-01 challenges)
	fallback := &mockTXTResolver{
		records: map[string][]string{
			"_acme-challenge.peerID.libp2p.direct": {"acme-token-value"},
		},
	}
	r := NewP2PForgeResolver([]string{"libp2p.direct"}, fallback)

	records, err := r.LookupTXT(t.Context(), "_acme-challenge.peerID.libp2p.direct")
	require.NoError(t, err)
	assert.Equal(t, []string{"acme-token-value"}, records)
}

func TestP2PForgeResolver_LookupTXT_EmptyFallback(t *testing.T) {
	r := NewP2PForgeResolver([]string{"libp2p.direct"}, &mockTXTResolver{})

	// When fallback returns nothing, we return nothing
	records, err := r.LookupTXT(t.Context(), "anything.libp2p.direct")
	require.NoError(t, err)
	assert.Empty(t, records)
}
