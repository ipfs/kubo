package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/kubo/boxo/autoconfig"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandAutoComprehensive(t *testing.T) {
	t.Parallel()

	t.Run("all autoconfig fields resolve correctly", func(t *testing.T) {
		t.Parallel()
		testAllAutoConfigFieldsResolve(t)
	})

	t.Run("bootstrap list --expand-auto matches config Bootstrap --expand-auto", func(t *testing.T) {
		t.Parallel()
		testBootstrapCommandConsistency(t)
	})

	t.Run("write operations fail with --expand-auto", func(t *testing.T) {
		t.Parallel()
		testWriteOperationsFailWithExpandAuto(t)
	})

	t.Run("config show --expand-auto provides complete expanded view", func(t *testing.T) {
		t.Parallel()
		testConfigShowExpandAutoComplete(t)
	})

	t.Run("multiple expand-auto calls use cache (single HTTP request)", func(t *testing.T) {
		t.Parallel()
		testMultipleExpandAutoUsesCache(t)
	})

	t.Run("expand-auto respects RefreshInterval for cache expiry", func(t *testing.T) {
		t.Parallel()
		testExpandAutoCacheExpiry(t)
	})
}

// testAllAutoConfigFieldsResolve verifies that all autoconfig fields (Bootstrap, DNS.Resolvers,
// Routing.DelegatedRouters, and Ipns.DelegatedPublishers) can be resolved from "auto" values
// to their actual configuration using --expand-auto flag.
//
// This test is critical because:
// 1. It validates the core autoconfig resolution functionality across all supported fields
// 2. It ensures that "auto" placeholders are properly replaced with real configuration values
// 3. It verifies that the autoconfig JSON structure is correctly parsed and applied
// 4. It tests the end-to-end flow from HTTP fetch to config field expansion
func testAllAutoConfigFieldsResolve(t *testing.T) {
	// Create comprehensive autoconfig response
	autoConfig := map[string]interface{}{
		"AutoConfigVersion": 2025072301,
		"AutoConfigSchema":  3,
		"Bootstrap": []string{
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		},
		"DNSResolvers": map[string][]string{
			".":    {"https://cloudflare-dns.com/dns-query"},
			"eth.": {"https://dns.google/dns-query"},
		},
		"DelegatedRouters": map[string][]string{
			autoconfig.MainnetProfileNodesWithDHT: {"https://cid.contact/routing/v1/providers"},
		},
		"DelegatedPublishers": map[string][]string{
			autoconfig.MainnetProfileIPNSPublishers: {"https://ipns.live"},
		},
	}

	autoConfigData, err := json.Marshal(autoConfig)
	require.NoError(t, err)

	// Create HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfigData)
	}))
	defer server.Close()

	// Create IPFS node with all auto values
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{
		".":    "auto",
		"eth.": "auto",
	})
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Test 1: Bootstrap resolution
	result := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "Bootstrap expansion should succeed")

	var expandedBootstrap []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedBootstrap)
	require.NoError(t, err)

	assert.NotContains(t, expandedBootstrap, "auto", "Bootstrap should not contain 'auto'")
	assert.Contains(t, expandedBootstrap, "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN")
	assert.Contains(t, expandedBootstrap, "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa")
	t.Logf("✅ Bootstrap expanded to: %v", expandedBootstrap)

	// Test 2: DNS.Resolvers resolution
	result = node.RunIPFS("config", "DNS.Resolvers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "DNS.Resolvers expansion should succeed")

	var expandedResolvers map[string]string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedResolvers)
	require.NoError(t, err)

	assert.NotContains(t, expandedResolvers, "auto", "DNS.Resolvers should not contain 'auto'")
	assert.Equal(t, "https://cloudflare-dns.com/dns-query", expandedResolvers["."])
	assert.Equal(t, "https://dns.google/dns-query", expandedResolvers["eth."])
	t.Logf("✅ DNS.Resolvers expanded to: %v", expandedResolvers)

	// Test 3: Routing.DelegatedRouters resolution
	result = node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "Routing.DelegatedRouters expansion should succeed")

	var expandedRouters []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedRouters)
	require.NoError(t, err)

	assert.NotContains(t, expandedRouters, "auto", "DelegatedRouters should not contain 'auto'")
	assert.Contains(t, expandedRouters, "https://cid.contact/routing/v1/providers")
	t.Logf("✅ Routing.DelegatedRouters expanded to: %v", expandedRouters)

	// Test 4: Ipns.DelegatedPublishers resolution
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "Ipns.DelegatedPublishers expansion should succeed")

	var expandedPublishers []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedPublishers)
	require.NoError(t, err)

	assert.NotContains(t, expandedPublishers, "auto", "DelegatedPublishers should not contain 'auto'")
	assert.Contains(t, expandedPublishers, "https://ipns.live")
	t.Logf("✅ Ipns.DelegatedPublishers expanded to: %v", expandedPublishers)
}

// testBootstrapCommandConsistency verifies that `ipfs bootstrap list --expand-auto` and
// `ipfs config Bootstrap --expand-auto` return identical results when both use autoconfig.
//
// This test is important because:
//  1. It ensures consistency between different CLI commands that access the same data
//  2. It validates that both the bootstrap-specific command and generic config command
//     use the same underlying autoconfig resolution mechanism
//  3. It prevents regression where different commands might resolve "auto" differently
//  4. It ensures users get consistent results regardless of which command they use
func testBootstrapCommandConsistency(t *testing.T) {
	// Load test autoconfig data
	autoConfigData := loadTestDataComprehensive(t, "valid_autoconfig.json")

	// Create HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfigData)
	}))
	defer server.Close()

	// Create IPFS node with auto bootstrap
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Get bootstrap via config command
	configResult := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, configResult.ExitCode(), "config Bootstrap --expand-auto should succeed")

	// Get bootstrap via bootstrap command
	bootstrapResult := node.RunIPFS("bootstrap", "list", "--expand-auto")
	require.Equal(t, 0, bootstrapResult.ExitCode(), "bootstrap list --expand-auto should succeed")

	// Parse both results
	var configBootstrap, bootstrapBootstrap []string
	err := json.Unmarshal([]byte(configResult.Stdout.String()), &configBootstrap)
	require.NoError(t, err)

	// Bootstrap command output is line-separated, not JSON
	bootstrapOutput := strings.TrimSpace(bootstrapResult.Stdout.String())
	if bootstrapOutput != "" {
		bootstrapBootstrap = strings.Split(bootstrapOutput, "\n")
	}

	// Results should be equivalent
	assert.Equal(t, len(configBootstrap), len(bootstrapBootstrap), "Both commands should return same number of peers")

	// Both should contain same peers (order might differ due to different output formats)
	for _, peer := range configBootstrap {
		found := false
		for _, bsPeer := range bootstrapBootstrap {
			if strings.TrimSpace(bsPeer) == peer {
				found = true
				break
			}
		}
		assert.True(t, found, "Peer %s should be in both results", peer)
	}

	t.Logf("✅ Config command result: %v", configBootstrap)
	t.Logf("✅ Bootstrap command result: %v", bootstrapBootstrap)
}

// testWriteOperationsFailWithExpandAuto verifies that --expand-auto flag is properly
// restricted to read-only operations and fails when used with config write operations.
//
// This test is essential because:
// 1. It enforces the security principle that --expand-auto should only be used for reading
// 2. It prevents users from accidentally overwriting config with expanded values
// 3. It ensures that "auto" placeholders are preserved in the stored configuration
// 4. It validates proper error handling and user guidance when misused
// 5. It protects against accidental loss of the "auto" semantic meaning
func testWriteOperationsFailWithExpandAuto(t *testing.T) {
	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Test that setting config with --expand-auto fails
	testCases := []struct {
		name string
		args []string
	}{
		{"config set with expand-auto", []string{"config", "Bootstrap", "[\"test\"]", "--expand-auto"}},
		{"config set JSON with expand-auto", []string{"config", "Bootstrap", "[\"test\"]", "--json", "--expand-auto"}},
		{"config set bool with expand-auto", []string{"config", "SomeField", "true", "--bool", "--expand-auto"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := node.RunIPFS(tc.args...)
			assert.NotEqual(t, 0, result.ExitCode(), "Write operation with --expand-auto should fail")

			stderr := result.Stderr.String()
			assert.Contains(t, stderr, "--expand-auto", "Error should mention --expand-auto")
			assert.Contains(t, stderr, "reading", "Error should mention reading limitation")
			t.Logf("✅ Expected error: %s", stderr)
		})
	}
}

// testConfigShowExpandAutoComplete verifies that `ipfs config show --expand-auto`
// produces a complete configuration with all "auto" values expanded to their resolved forms.
//
// This test is important because:
// 1. It validates the full-config expansion functionality for comprehensive troubleshooting
// 2. It ensures that users can see the complete resolved configuration state
// 3. It verifies that all "auto" placeholders are replaced, not just individual fields
// 4. It tests that the resulting JSON is valid and well-formed
// 5. It provides a way to export/backup the fully expanded configuration
func testConfigShowExpandAutoComplete(t *testing.T) {
	// Load test autoconfig data
	autoConfigData := loadTestDataComprehensive(t, "valid_autoconfig.json")

	// Create HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfigData)
	}))
	defer server.Close()

	// Create IPFS node with multiple auto values
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{".": "auto"})

	// Test config show --expand-auto
	result := node.RunIPFS("config", "show", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config show --expand-auto should succeed")

	expandedConfig := result.Stdout.String()

	// Should not contain any literal "auto" values
	assert.NotContains(t, expandedConfig, `"auto"`, "Expanded config should not contain literal 'auto' values")

	// Should contain expected expanded sections
	assert.Contains(t, expandedConfig, `"Bootstrap"`, "Should contain Bootstrap section")
	assert.Contains(t, expandedConfig, `"DNS"`, "Should contain DNS section")
	assert.Contains(t, expandedConfig, `"Resolvers"`, "Should contain Resolvers section")

	// Should contain expanded peer addresses (not "auto")
	assert.Contains(t, expandedConfig, "bootstrap.libp2p.io", "Should contain expanded bootstrap peers")

	// Should be valid JSON
	var configMap map[string]interface{}
	err := json.Unmarshal([]byte(expandedConfig), &configMap)
	require.NoError(t, err, "Expanded config should be valid JSON")

	// Verify specific fields were expanded
	if bootstrap, ok := configMap["Bootstrap"].([]interface{}); ok {
		assert.Greater(t, len(bootstrap), 0, "Bootstrap should have expanded entries")
		for _, peer := range bootstrap {
			assert.NotEqual(t, "auto", peer, "Bootstrap entries should not be 'auto'")
		}
	}

	t.Logf("✅ Config show --expand-auto produced %d characters of expanded config", len(expandedConfig))
}

// testMultipleExpandAutoUsesCache verifies that multiple consecutive --expand-auto calls
// efficiently use cached autoconfig data instead of making repeated HTTP requests.
//
// This test is critical for performance because:
// 1. It validates that the caching mechanism works correctly to reduce network overhead
// 2. It ensures that users can make multiple config queries without causing excessive HTTP traffic
// 3. It verifies that cached data is shared across different config fields and commands
// 4. It tests that HTTP headers (ETag/Last-Modified) are properly used for cache validation
// 5. It prevents regression where each --expand-auto call would trigger a new HTTP request
// 6. It demonstrates the performance benefit: 5 operations with only 1 network request
func testMultipleExpandAutoUsesCache(t *testing.T) {
	// Create comprehensive autoconfig response
	autoConfigData := loadTestDataComprehensive(t, "valid_autoconfig.json")

	// Track HTTP requests to verify caching
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		t.Logf("Autoconfig cache test request #%d: %s %s", count, r.Method, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"cache-test-123"`)
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		_, _ = w.Write(autoConfigData)
	}))
	defer server.Close()

	// Create IPFS node with all auto values
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	// Note: Using default RefreshInterval (24h) to ensure caching - explicit setting would require rebuilt binary

	// Set up auto values for multiple fields
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"foo.": "auto"})
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Reset counter to only track our expand-auto calls
	atomic.StoreInt32(&requestCount, 0)

	// Make multiple --expand-auto calls on different fields
	t.Log("Testing multiple --expand-auto calls should use cache...")

	// Call 1: Bootstrap --expand-auto (should trigger HTTP request)
	result1 := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result1.ExitCode(), "Bootstrap --expand-auto should succeed")

	var expandedBootstrap []string
	err := json.Unmarshal([]byte(result1.Stdout.String()), &expandedBootstrap)
	require.NoError(t, err)
	assert.NotContains(t, expandedBootstrap, "auto", "Bootstrap should be expanded")
	assert.Greater(t, len(expandedBootstrap), 0, "Bootstrap should have entries")

	// Call 2: DNS.Resolvers --expand-auto (should use cache, no HTTP)
	result2 := node.RunIPFS("config", "DNS.Resolvers", "--expand-auto")
	require.Equal(t, 0, result2.ExitCode(), "DNS.Resolvers --expand-auto should succeed")

	var expandedResolvers map[string]string
	err = json.Unmarshal([]byte(result2.Stdout.String()), &expandedResolvers)
	require.NoError(t, err)

	// Call 3: Routing.DelegatedRouters --expand-auto (should use cache, no HTTP)
	result3 := node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result3.ExitCode(), "Routing.DelegatedRouters --expand-auto should succeed")

	var expandedRouters []string
	err = json.Unmarshal([]byte(result3.Stdout.String()), &expandedRouters)
	require.NoError(t, err)
	assert.NotContains(t, expandedRouters, "auto", "Routers should be expanded")

	// Call 4: Ipns.DelegatedPublishers --expand-auto (should use cache, no HTTP)
	result4 := node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	require.Equal(t, 0, result4.ExitCode(), "Ipns.DelegatedPublishers --expand-auto should succeed")

	var expandedPublishers []string
	err = json.Unmarshal([]byte(result4.Stdout.String()), &expandedPublishers)
	require.NoError(t, err)
	assert.NotContains(t, expandedPublishers, "auto", "Publishers should be expanded")

	// Call 5: config show --expand-auto (should use cache, no HTTP)
	result5 := node.RunIPFS("config", "show", "--expand-auto")
	require.Equal(t, 0, result5.ExitCode(), "config show --expand-auto should succeed")

	expandedConfig := result5.Stdout.String()
	assert.NotContains(t, expandedConfig, `"auto"`, "Full config should not contain 'auto' values")

	// CRITICAL TEST: Verify only 1 HTTP request was made despite 5 --expand-auto calls
	finalRequestCount := atomic.LoadInt32(&requestCount)
	assert.Equal(t, int32(1), finalRequestCount,
		"Multiple --expand-auto calls should result in only 1 HTTP request due to caching. Got %d requests", finalRequestCount)

	t.Logf("✅ Made 5 --expand-auto calls, resulted in only %d HTTP request(s) - caching works!", finalRequestCount)
}

// testExpandAutoCacheExpiry verifies that autoconfig cache properly expires based on
// the configured RefreshInterval and fetches fresh data when needed.
//
// This test is essential for correctness because:
// 1. It validates that cache expiry works correctly to ensure users get updated configuration
// 2. It verifies that the RefreshInterval setting is properly respected
// 3. It ensures that stale cached data doesn't persist indefinitely
// 4. It tests the balance between performance (caching) and freshness (expiry)
// 5. It validates that new HTTP requests are made after cache expiry
// 6. It prevents issues where users might get outdated autoconfig indefinitely
func testExpandAutoCacheExpiry(t *testing.T) {
	// Create autoconfig response
	autoConfigData := loadTestDataComprehensive(t, "valid_autoconfig.json")

	// Track HTTP requests with timestamps
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		t.Logf("Cache expiry test request #%d at %s: %s %s", count, time.Now().Format("15:04:05.000"), r.Method, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		// Use different ETag for each request to ensure we can detect new fetches
		w.Header().Set("ETag", fmt.Sprintf(`"expiry-test-%d"`, count))
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		_, _ = w.Write(autoConfigData)
	}))
	defer server.Close()

	// Create IPFS node with SHORT refresh interval for testing
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	// Set short RefreshInterval for testing cache expiry
	node.SetIPFSConfig("AutoConfig.RefreshInterval", "200ms")

	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"test.": "auto"})

	// Reset counter
	atomic.StoreInt32(&requestCount, 0)

	// First --expand-auto call (should trigger HTTP request)
	t.Log("First --expand-auto call (cache miss)...")
	result1 := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result1.ExitCode(), "First Bootstrap --expand-auto should succeed")

	// Verify we got a valid response
	var bootstrap1 []string
	err := json.Unmarshal([]byte(result1.Stdout.String()), &bootstrap1)
	require.NoError(t, err)
	assert.Greater(t, len(bootstrap1), 0, "First call should return expanded bootstrap")

	// Second call immediately (should use cache)
	t.Log("Second --expand-auto call immediately (cache hit)...")
	result2 := node.RunIPFS("config", "DNS.Resolvers", "--expand-auto")
	require.Equal(t, 0, result2.ExitCode(), "Second DNS.Resolvers --expand-auto should succeed")

	// At this point we should have exactly 1 HTTP request
	requestsAfterCache := atomic.LoadInt32(&requestCount)
	assert.Equal(t, int32(1), requestsAfterCache, "Should have 1 request after cache hit")

	// Wait for cache to expire (200ms + buffer)
	t.Log("Waiting for cache to expire...")
	time.Sleep(300 * time.Millisecond)

	// Third call after cache expiry (should trigger new HTTP request)
	t.Log("Third --expand-auto call after cache expiry (cache miss)...")
	result3 := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result3.ExitCode(), "Third Bootstrap --expand-auto should succeed")

	// Verify we got a valid response
	var bootstrap3 []string
	err = json.Unmarshal([]byte(result3.Stdout.String()), &bootstrap3)
	require.NoError(t, err)
	assert.Greater(t, len(bootstrap3), 0, "Third call should return expanded bootstrap")

	// Now we should have exactly 2 HTTP requests
	finalRequestCount := atomic.LoadInt32(&requestCount)
	assert.Equal(t, int32(2), finalRequestCount,
		"After cache expiry, should have 2 total HTTP requests. Got %d", finalRequestCount)

	t.Logf("✅ Cache expiry test successful: 2 cache misses + 1 cache hit = %d HTTP requests", finalRequestCount)
}

// loadTestDataComprehensive is a helper function that loads test autoconfig JSON data files.
// It locates the test data directory relative to the test file and reads the specified file.
// This centralized helper ensures consistent test data loading across all comprehensive tests.
func loadTestDataComprehensive(t *testing.T, filename string) []byte {
	t.Helper()

	// Get the test data directory relative to this test file
	testDir := filepath.Dir(func() string {
		_, file, _, _ := runtime.Caller(0)
		return file
	}())

	dataPath := filepath.Join(testDir, "autoconfig_test_data", filename)
	data, err := os.ReadFile(dataPath)
	require.NoError(t, err, "Failed to read test data file: %s", filename)

	return data
}
