package libp2p

import (
	"context"
	"net"
	"testing"

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBasicResolver implements madns.BasicResolver for testing
type mockBasicResolver struct {
	txtRecords map[string][]string
	ipRecords  map[string][]net.IPAddr
}

func (m *mockBasicResolver) LookupIPAddr(ctx context.Context, name string) ([]net.IPAddr, error) {
	if m.ipRecords != nil {
		return m.ipRecords[name], nil
	}
	return nil, nil
}

func (m *mockBasicResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if m.txtRecords != nil {
		return m.txtRecords[name], nil
	}
	return nil, nil
}

func TestNewNetResolverFromMadns_LookupTXT(t *testing.T) {
	// Create mock resolver with known TXT records
	mock := &mockBasicResolver{
		txtRecords: map[string][]string{
			"_acme-challenge.peer.libp2p.direct": {"test-acme-token-12345"},
			"_dnslink.example.com":               {"dnslink=/ipfs/QmTest"},
		},
	}

	// Create madns resolver with mock as default
	madnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(mock))
	require.NoError(t, err)

	// Create net.Resolver via our bridge
	netResolver := NewNetResolverFromMadns(madnsResolver)

	// Test TXT lookup
	records, err := netResolver.LookupTXT(t.Context(), "_acme-challenge.peer.libp2p.direct")
	require.NoError(t, err)
	assert.Equal(t, []string{"test-acme-token-12345"}, records)

	// Test another domain
	records, err = netResolver.LookupTXT(t.Context(), "_dnslink.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"dnslink=/ipfs/QmTest"}, records)

	// Test non-existent domain - Go's net.Resolver returns error for empty responses
	records, err = netResolver.LookupTXT(t.Context(), "nonexistent.example.com")
	// net.Resolver interprets empty authoritative response as "no such host"
	require.Error(t, err)
	assert.Empty(t, records)
}

func TestNewNetResolverFromMadns_LookupIP(t *testing.T) {
	t.Run("returns both IPv4 and IPv6", func(t *testing.T) {
		mock := &mockBasicResolver{
			ipRecords: map[string][]net.IPAddr{
				"example.com": {
					{IP: net.ParseIP("192.168.1.1")},
					{IP: net.ParseIP("2001:db8::1")},
				},
			},
		}
		madnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(mock))
		require.NoError(t, err)

		netResolver := NewNetResolverFromMadns(madnsResolver)
		ips, err := netResolver.LookupIP(t.Context(), "ip", "example.com")
		require.NoError(t, err)
		assert.Len(t, ips, 2)
	})

	t.Run("IPv4 only", func(t *testing.T) {
		mock := &mockBasicResolver{
			ipRecords: map[string][]net.IPAddr{
				"ipv4only.example.com": {
					{IP: net.ParseIP("10.0.0.1")},
					{IP: net.ParseIP("10.0.0.2")},
				},
			},
		}
		madnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(mock))
		require.NoError(t, err)

		netResolver := NewNetResolverFromMadns(madnsResolver)
		ips, err := netResolver.LookupIP(t.Context(), "ip4", "ipv4only.example.com")
		require.NoError(t, err)
		assert.Len(t, ips, 2)
		for _, ip := range ips {
			assert.NotNil(t, ip.To4(), "expected IPv4 address")
		}
	})

	t.Run("IPv6 only", func(t *testing.T) {
		mock := &mockBasicResolver{
			ipRecords: map[string][]net.IPAddr{
				"ipv6only.example.com": {
					{IP: net.ParseIP("2001:db8::1")},
					{IP: net.ParseIP("2001:db8::2")},
				},
			},
		}
		madnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(mock))
		require.NoError(t, err)

		netResolver := NewNetResolverFromMadns(madnsResolver)
		ips, err := netResolver.LookupIP(t.Context(), "ip6", "ipv6only.example.com")
		require.NoError(t, err)
		assert.Len(t, ips, 2)
		for _, ip := range ips {
			assert.Nil(t, ip.To4(), "expected IPv6 address")
		}
	})

	t.Run("non-existent domain returns error", func(t *testing.T) {
		mock := &mockBasicResolver{}
		madnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(mock))
		require.NoError(t, err)

		netResolver := NewNetResolverFromMadns(madnsResolver)
		ips, err := netResolver.LookupIP(t.Context(), "ip", "nonexistent.example.com")
		// net.Resolver returns error for empty authoritative response
		require.Error(t, err)
		assert.Empty(t, ips)
	})
}

func TestNewNetResolverFromMadns_MultipleTXTRecords(t *testing.T) {
	mock := &mockBasicResolver{
		txtRecords: map[string][]string{
			"multi.example.com": {"value1", "value2", "value3"},
		},
	}
	madnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(mock))
	require.NoError(t, err)

	netResolver := NewNetResolverFromMadns(madnsResolver)
	records, err := netResolver.LookupTXT(t.Context(), "multi.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"value1", "value2", "value3"}, records)
}
