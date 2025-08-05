package autoconf

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRealAutoConfURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "autoconf-integration-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(
		WithCacheDir(tmpDir),
		WithUserAgent("kubo-autoconf-test/1.0"),
		WithTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Test with the real autoconf URL
	resp, err := client.GetLatest(ctx, MainnetAutoConfURL, DefaultRefreshInterval)
	if err != nil {
		t.Fatalf("failed to get real autoconf: %v", err)
	}
	config := resp.Config

	// Verify the config structure
	if config.AutoConfVersion == 0 {
		t.Error("expected non-zero AutoConfVersion")
	}
	if config.AutoConfSchema == 0 {
		t.Error("expected non-zero AutoConfSchema")
	}

	// Get bootstrap peers from all systems to verify
	bootstrapPeers := config.GetBootstrapPeers(SystemAminoDHT)
	if len(bootstrapPeers) == 0 {
		t.Error("expected non-empty bootstrap peers")
	}

	t.Logf("Successfully fetched autoconf version %d with schema %d",
		config.AutoConfVersion, config.AutoConfSchema)
	t.Logf("Bootstrap peers: %d", len(bootstrapPeers))
	t.Logf("DNS resolvers: %d", len(config.DNSResolvers))
	t.Logf("System registry: %d", len(config.SystemRegistry))
	t.Logf("Delegated endpoints: %d", len(config.DelegatedEndpoints))

	// Test cache functionality by fetching again
	resp2, err := client.GetLatest(ctx, MainnetAutoConfURL, DefaultRefreshInterval)
	if err != nil {
		t.Fatalf("failed to get cached autoconf: %v", err)
	}

	config2 := resp2.Config
	if config2.AutoConfVersion != config.AutoConfVersion {
		t.Errorf("cache version mismatch: expected %d, got %d",
			config.AutoConfVersion, config2.AutoConfVersion)
	}
}
