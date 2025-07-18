package autoconfig

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ipfs/kubo"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
}

func TestWithOptions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "autoconfig-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(
		WithCacheDir(tmpDir),
		WithCacheSize(5),
		WithUserAgent(ipfs.GetUserAgentVersion()),
		WithTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if client.cacheDir != tmpDir {
		t.Errorf("expected cache dir %s, got %s", tmpDir, client.cacheDir)
	}
	if client.cacheSize != 5 {
		t.Errorf("expected cache size 5, got %d", client.cacheSize)
	}
	if client.httpClient.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", client.httpClient.Timeout)
	}
}

func TestGetLatest(t *testing.T) {
	// Create test config
	testConfig := &AutoConfig{
		AutoConfigVersion: 2025071802,
		AutoConfigSchema:  2,
		Bootstrap:         []string{"/ip4/127.0.0.1/tcp/4001/p2p/QmTest"},
		DNSResolvers:      map[string][]string{"eth.": {"https://example.com"}},
		DelegatedRouters: map[string]DelegatedRouterConfig{
			"for-nodes-with-dht": {
				Providers: []string{"https://cid.contact"},
			},
		},
		DelegatedPublishers: map[string]DelegatedPublisherConfig{
			"for-ipns-publishers-with-http": {
				IPNS: []string{"https://delegated-ipfs.dev"},
			},
		},
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "\"test-etag\"")
		json.NewEncoder(w).Encode(testConfig)
	}))
	defer server.Close()

	// Create client
	tmpDir, err := os.MkdirTemp("", "autoconfig-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(
		WithCacheDir(tmpDir),
		WithUserAgent(ipfs.GetUserAgentVersion()),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Test fetching
	ctx := context.Background()
	config, err := client.GetLatest(ctx, server.URL)
	if err != nil {
		t.Fatalf("failed to get latest config: %v", err)
	}

	if config.AutoConfigVersion != testConfig.AutoConfigVersion {
		t.Errorf("expected version %d, got %d", testConfig.AutoConfigVersion, config.AutoConfigVersion)
	}

	// Verify cache was created
	cacheDir, err := client.getCacheDir(server.URL)
	if err != nil {
		t.Fatalf("failed to get cache dir: %v", err)
	}

	cachedFile := filepath.Join(cacheDir, "2025071802.json")
	if _, err := os.Stat(cachedFile); os.IsNotExist(err) {
		t.Errorf("expected cache file %s to exist", cachedFile)
	}
}

func TestCacheMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "autoconfig-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(WithCacheDir(tmpDir))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "autoconfig", "example.com")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	// Test writing metadata
	err = client.writeCachedMetadata(cacheDir, "test-etag", "test-lastmod")
	if err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	// Test reading metadata
	etag, lastMod := client.readCachedMetadata(cacheDir)
	if etag != "test-etag" {
		t.Errorf("expected etag 'test-etag', got '%s'", etag)
	}
	if lastMod != "test-lastmod" {
		t.Errorf("expected last modified 'test-lastmod', got '%s'", lastMod)
	}
}
