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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestWithOptions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "autoconf-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(
		WithCacheDir(tmpDir),
		WithCacheSize(5),
		WithUserAgent("kubo-autoconf-test/1.0"),
		WithTimeout(10*time.Second),
	)
	require.NoError(t, err)

	assert.Equal(t, tmpDir, client.cacheDir)
	assert.Equal(t, 5, client.cacheSize)
	assert.Equal(t, 10*time.Second, client.httpClient.Timeout)
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

	// Create client with test server URL
	tmpDir, err := os.MkdirTemp("", "autoconf-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(
		WithCacheDir(tmpDir),
		WithUserAgent("kubo-autoconf-test/1.0"),
		WithURL(server.URL),
		WithRefreshInterval(time.Hour),
	)
	require.NoError(t, err)

	// Test fetching
	ctx := context.Background()
	response, err := client.GetLatest(ctx)
	require.NoError(t, err)

	assert.Equal(t, testConfig.AutoConfVersion, response.Config.AutoConfVersion)

	// Verify cache was created
	cacheDir, err := client.getCacheDir()
	require.NoError(t, err)

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
	assert.True(t, foundCacheFile, "expected some autoconf cache file to exist in %s", cacheDir)
}

func TestCacheMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "autoconf-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	client, err := NewClient(WithCacheDir(tmpDir))
	require.NoError(t, err)

	cacheDir := filepath.Join(tmpDir, "autoconf", "example.com")
	err = os.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)

	// Test writing metadata directly (since writeMetadata is now inlined in saveToCache)
	err = writeOwnerOnlyFile(filepath.Join(cacheDir, etagFile), []byte("test-etag"))
	require.NoError(t, err)
	err = writeOwnerOnlyFile(filepath.Join(cacheDir, lastModifiedFile), []byte("test-lastmod"))
	require.NoError(t, err)

	// Test reading metadata
	etag, lastMod := client.readMetadata(cacheDir)
	assert.Equal(t, "test-etag", etag)
	assert.Equal(t, "test-lastmod", lastMod)
}
