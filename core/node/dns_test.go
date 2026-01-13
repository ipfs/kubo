package node

import (
	"context"
	"net"
	"testing"

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockResolver implements madns.BasicResolver for testing
type mockResolver struct {
	txtRecords map[string][]string
	ipRecords  map[string][]net.IPAddr
}

func (m *mockResolver) LookupIPAddr(ctx context.Context, name string) ([]net.IPAddr, error) {
	if m.ipRecords != nil {
		return m.ipRecords[name], nil
	}
	return nil, nil
}

func (m *mockResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if m.txtRecords != nil {
		return m.txtRecords[name], nil
	}
	return nil, nil
}

func TestOverrideDefaultResolver(t *testing.T) {
	// Save original resolver to restore after test
	originalResolver := net.DefaultResolver
	t.Cleanup(func() {
		net.DefaultResolver = originalResolver
	})

	// Create mock with known records
	mock := &mockResolver{
		txtRecords: map[string][]string{
			"test.override.example": {"override-test-value"},
		},
	}

	madnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(mock))
	require.NoError(t, err)

	// Override the default resolver
	OverrideDefaultResolver(madnsResolver)

	// Verify net.DefaultResolver now uses our mock
	records, err := net.DefaultResolver.LookupTXT(t.Context(), "test.override.example")
	require.NoError(t, err)
	assert.Equal(t, []string{"override-test-value"}, records)
}
