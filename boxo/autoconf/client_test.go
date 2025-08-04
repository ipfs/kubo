package autoconf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	tmpDir, err := os.MkdirTemp("", "autoconf-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(
		WithCacheDir(tmpDir),
		WithCacheSize(5),
		WithUserAgent("kubo-autoconf-test/1.0"),
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
	testConfig := &Config{
		AutoConfVersion: 2025071802,
		AutoConfSchema:  1,
		AutoConfTTL:     86400,
		SystemRegistry: map[string]SystemConfig{
			SystemAminoDHT: {
				Description: "Test AminoDHT system",
				NativeConfig: &NativeConfig{
					Bootstrap: []string{"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"},
				},
				DelegatedConfig: &DelegatedConfig{
					Read:  []string{"/routing/v1/providers"},
					Write: []string{},
				},
			},
		},
		DNSResolvers: map[string][]string{"eth.": {"https://example.com"}},
		DelegatedEndpoints: map[string]EndpointConfig{
			"https://ipni.example.com": {
				Systems: []string{SystemIPNI},
				Read:    []string{"/routing/v1/providers"},
				Write:   []string{},
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
	tmpDir, err := os.MkdirTemp("", "autoconf-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(
		WithCacheDir(tmpDir),
		WithUserAgent("kubo-autoconf-test/1.0"),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Test fetching
	ctx := context.Background()
	response, err := client.GetLatest(ctx, server.URL, time.Hour)
	if err != nil {
		t.Fatalf("failed to get latest config: %v", err)
	}

	if response.Config.AutoConfVersion != testConfig.AutoConfVersion {
		t.Errorf("expected version %d, got %d", testConfig.AutoConfVersion, response.Config.AutoConfVersion)
	}

	// Verify cache was created
	cacheDir, err := client.getCacheDir(server.URL)
	if err != nil {
		t.Fatalf("failed to get cache dir: %v", err)
	}

	// List files in cache dir for debugging
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Logf("failed to read cache dir %s: %v", cacheDir, err)
	} else {
		t.Logf("cache dir %s contains: %v", cacheDir, files)
	}

	// Check that some autoconf JSON file exists (filename may vary based on timestamp)
	foundCacheFile := false
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") && strings.Contains(file.Name(), "autoconf") {
			foundCacheFile = true
			t.Logf("found cache file: %s", file.Name())
			break
		}
	}
	if !foundCacheFile {
		t.Errorf("expected some autoconf cache file to exist in %s, but none found", cacheDir)
	}
}

func TestCacheMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "autoconf-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(WithCacheDir(tmpDir))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	cacheDir := filepath.Join(tmpDir, "autoconf", "example.com")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	// Test writing metadata directly (since writeMetadata is now inlined in saveToCache)
	if err := writeOwnerOnlyFile(filepath.Join(cacheDir, etagFile), []byte("test-etag")); err != nil {
		t.Fatalf("failed to write etag: %v", err)
	}
	if err := writeOwnerOnlyFile(filepath.Join(cacheDir, lastModifiedFile), []byte("test-lastmod")); err != nil {
		t.Fatalf("failed to write last-modified: %v", err)
	}

	// Test reading metadata
	etag, lastMod := client.readMetadata(cacheDir)
	if etag != "test-etag" {
		t.Errorf("expected etag 'test-etag', got '%s'", etag)
	}
	if lastMod != "test-lastmod" {
		t.Errorf("expected last modified 'test-lastmod', got '%s'", lastMod)
	}
}
