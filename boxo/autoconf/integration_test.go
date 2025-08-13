package autoconf

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealAutoConfURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "autoconf-integration-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(
		WithCacheDir(tmpDir),
		WithUserAgent("kubo-autoconf-test/1.0"),
		WithTimeout(10*time.Second),
		WithURL(MainnetAutoConfURL),
		WithRefreshInterval(DefaultRefreshInterval),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Test with the real autoconf URL
	resp, err := client.GetLatest(ctx)
	require.NoError(t, err)
	config := resp.Config

	// Verify the config structure
	assert.NotZero(t, config.AutoConfVersion, "expected non-zero AutoConfVersion")
	assert.NotZero(t, config.AutoConfSchema, "expected non-zero AutoConfSchema")

	// Get bootstrap peers from all systems to verify
	bootstrapPeers := config.GetBootstrapPeers(SystemAminoDHT)
	assert.NotEmpty(t, bootstrapPeers, "expected non-empty bootstrap peers")

	t.Logf("Successfully fetched autoconf version %d with schema %d",
		config.AutoConfVersion, config.AutoConfSchema)
	t.Logf("Bootstrap peers: %d", len(bootstrapPeers))
	t.Logf("DNS resolvers: %d", len(config.DNSResolvers))
	t.Logf("System registry: %d", len(config.SystemRegistry))
	t.Logf("Delegated endpoints: %d", len(config.DelegatedEndpoints))

	// Test cache functionality by fetching again
	resp2, err := client.GetLatest(ctx)
	require.NoError(t, err)

	config2 := resp2.Config
	assert.Equal(t, config.AutoConfVersion, config2.AutoConfVersion,
		"cache version mismatch")
}
