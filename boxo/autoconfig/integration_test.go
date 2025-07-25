package autoconfig

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRealAutoConfigURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "autoconfig-integration-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(
		WithCacheDir(tmpDir),
		WithUserAgent("kubo-autoconfig-test/1.0"),
		WithTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Test with the real autoconfig URL
	resp, err := client.GetLatest(ctx, MainnetAutoConfigURL, DefaultRefreshInterval)
	if err != nil {
		t.Fatalf("failed to get real autoconfig: %v", err)
	}
	config := resp.Config

	// Verify the config structure
	if config.AutoConfigVersion == 0 {
		t.Error("expected non-zero AutoConfigVersion")
	}
	if config.AutoConfigSchema == 0 {
		t.Error("expected non-zero AutoConfigSchema")
	}
	if len(config.Bootstrap) == 0 {
		t.Error("expected non-empty Bootstrap")
	}

	t.Logf("Successfully fetched autoconfig version %d with schema %d",
		config.AutoConfigVersion, config.AutoConfigSchema)
	t.Logf("Bootstrap peers: %d", len(config.Bootstrap))
	t.Logf("DNS resolvers: %d", len(config.DNSResolvers))
	t.Logf("Delegated routers: %d", len(config.DelegatedRouters))
	t.Logf("Delegated publishers: %d", len(config.DelegatedPublishers))

	// Test cache functionality by fetching again
	resp2, err := client.GetLatest(ctx, MainnetAutoConfigURL, DefaultRefreshInterval)
	if err != nil {
		t.Fatalf("failed to get cached autoconfig: %v", err)
	}

	config2 := resp2.Config
	if config2.AutoConfigVersion != config.AutoConfigVersion {
		t.Errorf("cache version mismatch: expected %d, got %d",
			config.AutoConfigVersion, config2.AutoConfigVersion)
	}
}
