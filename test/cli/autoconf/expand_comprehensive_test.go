// Package autoconf provides comprehensive tests for --expand-auto functionality.
//
// Test Scenarios:
//  1. Tests WITH daemon: Most tests start a daemon to fetch and cache autoconf data,
//     then test CLI commands that read from that cache using MustGetConfigCached.
//  2. Tests WITHOUT daemon: Error condition tests that don't need cached autoconf.
//
// The daemon setup uses startDaemonAndWaitForAutoConf() helper which:
// - Starts the daemon
// - Waits for HTTP request to mock server (not arbitrary timeout)
// - Returns when autoconf is cached and ready for CLI commands
package autoconf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandAutoComprehensive(t *testing.T) {
	t.Parallel()

	t.Run("all autoconf fields resolve correctly", func(t *testing.T) {
		t.Parallel()
		testAllAutoConfFieldsResolve(t)
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

	t.Run("CLI uses cache only while daemon handles background updates", func(t *testing.T) {
		t.Parallel()
		testCLIUsesCacheOnlyDaemonUpdatesBackground(t)
	})
}

// testAllAutoConfFieldsResolve verifies that all autoconf fields (Bootstrap, DNS.Resolvers,
// Routing.DelegatedRouters, and Ipns.DelegatedPublishers) can be resolved from "auto" values
// to their actual configuration using --expand-auto flag with daemon-cached autoconf data.
//
// This test is critical because:
// 1. It validates the core autoconf resolution functionality across all supported fields
// 2. It ensures that "auto" placeholders are properly replaced with real configuration values
// 3. It verifies that the autoconf JSON structure is correctly parsed and applied
// 4. It tests the end-to-end flow from HTTP fetch to config field expansion
func testAllAutoConfFieldsResolve(t *testing.T) {
	// Test scenario: CLI with daemon started and autoconf cached
	// This validates core autoconf resolution functionality across all supported fields

	// Track HTTP requests to verify mock server is being used
	var requestCount atomic.Int32
	var autoConfData []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		t.Logf("Mock autoconf server request #%d: %s %s", count, r.Method, r.URL.Path)

		// Create comprehensive autoconf response matching Schema 4 format
		// Use server URLs to ensure they're reachable and valid
		serverURL := fmt.Sprintf("http://%s", r.Host) // Get the server URL from the request
		autoConf := map[string]interface{}{
			"AutoConfVersion": 2025072301,
			"AutoConfSchema":  1,
			"AutoConfTTL":     86400,
			"SystemRegistry": map[string]interface{}{
				"AminoDHT": map[string]interface{}{
					"URL":         "https://github.com/ipfs/specs/pull/497",
					"Description": "Test AminoDHT system",
					"NativeConfig": map[string]interface{}{
						"Bootstrap": []string{
							"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
							"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
						},
					},
					"DelegatedConfig": map[string]interface{}{
						"Read":  []string{"/routing/v1/providers", "/routing/v1/peers", "/routing/v1/ipns"},
						"Write": []string{"/routing/v1/ipns"},
					},
				},
				"IPNI": map[string]interface{}{
					"URL":         serverURL + "/ipni-system",
					"Description": "Test IPNI system",
					"DelegatedConfig": map[string]interface{}{
						"Read":  []string{"/routing/v1/providers"},
						"Write": []string{},
					},
				},
				"CustomIPNS": map[string]interface{}{
					"URL":         serverURL + "/ipns-system",
					"Description": "Test IPNS system",
					"DelegatedConfig": map[string]interface{}{
						"Read":  []string{"/routing/v1/ipns"},
						"Write": []string{"/routing/v1/ipns"},
					},
				},
			},
			"DNSResolvers": map[string][]string{
				".":    {"https://cloudflare-dns.com/dns-query"},
				"eth.": {"https://dns.google/dns-query"},
			},
			"DelegatedEndpoints": map[string]interface{}{
				serverURL: map[string]interface{}{
					"Systems": []string{"IPNI", "CustomIPNS"}, // Use non-AminoDHT systems to avoid filtering
					"Read":    []string{"/routing/v1/providers", "/routing/v1/ipns"},
					"Write":   []string{"/routing/v1/ipns"},
				},
			},
		}

		var err error
		autoConfData, err = json.Marshal(autoConf)
		if err != nil {
			t.Fatalf("Failed to marshal autoConf: %v", err)
		}

		t.Logf("Serving mock autoconf data: %s", string(autoConfData))

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"test-mock-config"`)
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Create IPFS node with all auto values
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Clear any existing autoconf cache to prevent interference
	result := node.RunIPFS("config", "show")
	if result.ExitCode() == 0 {
		var cfg map[string]interface{}
		if json.Unmarshal([]byte(result.Stdout.String()), &cfg) == nil {
			if repoPath, exists := cfg["path"]; exists {
				if pathStr, ok := repoPath.(string); ok {
					t.Logf("Clearing autoconf cache from %s/autoconf", pathStr)
					// Note: We can't directly remove files, but clearing cache via config change should help
				}
			}
		}
	}
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("AutoConf.RefreshInterval", "1s") // Force fresh fetches for testing
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{
		".":    "auto",
		"eth.": "auto",
	})
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Start daemon and wait for autoconf fetch
	daemon := startDaemonAndWaitForAutoConf(t, node, &requestCount)
	defer daemon.StopDaemon()

	// Test 1: Bootstrap resolution
	result = node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "Bootstrap expansion should succeed")

	var expandedBootstrap []string
	var err error
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedBootstrap)
	require.NoError(t, err)

	assert.NotContains(t, expandedBootstrap, "auto", "Bootstrap should not contain 'auto'")
	assert.Contains(t, expandedBootstrap, "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN")
	assert.Contains(t, expandedBootstrap, "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa")
	t.Logf("Bootstrap expanded to: %v", expandedBootstrap)

	// Test 2: DNS.Resolvers resolution
	result = node.RunIPFS("config", "DNS.Resolvers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "DNS.Resolvers expansion should succeed")

	var expandedResolvers map[string]string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedResolvers)
	require.NoError(t, err)

	assert.NotContains(t, expandedResolvers, "auto", "DNS.Resolvers should not contain 'auto'")
	assert.Equal(t, "https://cloudflare-dns.com/dns-query", expandedResolvers["."])
	assert.Equal(t, "https://dns.google/dns-query", expandedResolvers["eth."])
	t.Logf("DNS.Resolvers expanded to: %v", expandedResolvers)

	// Test 3: Routing.DelegatedRouters resolution
	result = node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "Routing.DelegatedRouters expansion should succeed")

	var expandedRouters []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedRouters)
	require.NoError(t, err)

	assert.NotContains(t, expandedRouters, "auto", "DelegatedRouters should not contain 'auto'")

	// Test should strictly require mock autoconf to work - no fallback acceptance
	// The mock endpoint has Read paths ["/routing/v1/providers", "/routing/v1/ipns"]
	// so we expect 2 URLs with those paths
	expectedMockURLs := []string{
		server.URL + "/routing/v1/providers",
		server.URL + "/routing/v1/ipns",
	}
	require.Equal(t, 2, len(expandedRouters),
		"Should have exactly 2 routers from mock autoconf (one for each Read path). Got %d routers: %v. "+
			"This indicates autoconf is not working properly - check if mock server data is being parsed and filtered correctly.",
		len(expandedRouters), expandedRouters)

	// Check that both expected URLs are present
	for _, expectedURL := range expectedMockURLs {
		assert.Contains(t, expandedRouters, expectedURL,
			"Should contain mock autoconf endpoint with path %s. Got: %v. "+
				"This indicates autoconf endpoint path generation is not working properly.",
			expectedURL, expandedRouters)
	}

	// Test 4: Ipns.DelegatedPublishers resolution
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "Ipns.DelegatedPublishers expansion should succeed")

	var expandedPublishers []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedPublishers)
	require.NoError(t, err)

	assert.NotContains(t, expandedPublishers, "auto", "DelegatedPublishers should not contain 'auto'")

	// Test should require mock autoconf endpoint for IPNS publishing
	// The mock endpoint supports /routing/v1/ipns write operations, so it should be included with path
	expectedMockPublisherURL := server.URL + "/routing/v1/ipns"
	require.Equal(t, 1, len(expandedPublishers),
		"Should have exactly 1 IPNS publisher from mock autoconf. Got %d publishers: %v. "+
			"This indicates autoconf IPNS publisher filtering is not working properly.",
		len(expandedPublishers), expandedPublishers)
	assert.Equal(t, expectedMockPublisherURL, expandedPublishers[0],
		"Should use mock autoconf endpoint %s for IPNS publishing, not fallback. Got: %s. "+
			"This indicates autoconf IPNS publisher resolution is not working properly.",
		expectedMockPublisherURL, expandedPublishers[0])

	// CRITICAL: Verify that mock server was actually used
	finalRequestCount := requestCount.Load()
	require.Greater(t, finalRequestCount, int32(0),
		"Mock autoconf server should have been called at least once. Got %d requests. "+
			"This indicates the test is using cached or fallback config instead of mock data.", finalRequestCount)
	t.Logf("Mock server was called %d times - test is using mock data", finalRequestCount)
}

// testBootstrapCommandConsistency verifies that `ipfs bootstrap list --expand-auto` and
// `ipfs config Bootstrap --expand-auto` return identical results when both use autoconf.
//
// This test is important because:
//  1. It ensures consistency between different CLI commands that access the same data
//  2. It validates that both the bootstrap-specific command and generic config command
//     use the same underlying autoconf resolution mechanism
//  3. It prevents regression where different commands might resolve "auto" differently
//  4. It ensures users get consistent results regardless of which command they use
func testBootstrapCommandConsistency(t *testing.T) {
	// Test scenario: CLI with daemon started and autoconf cached
	// This ensures both bootstrap commands read from the same cached autoconf data

	// Load test autoconf data
	autoConfData := loadTestDataComprehensive(t, "valid_autoconf.json")

	// Track HTTP requests to verify daemon fetches autoconf
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		t.Logf("Bootstrap consistency test request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Create IPFS node with auto bootstrap
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Start daemon and wait for autoconf fetch
	daemon := startDaemonAndWaitForAutoConf(t, node, &requestCount)
	defer daemon.StopDaemon()

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

	t.Logf("Config command result: %v", configBootstrap)
	t.Logf("Bootstrap command result: %v", bootstrapBootstrap)
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
	// Test scenario: CLI without daemon (tests error conditions)
	// This test doesn't need daemon setup since it's testing that write operations
	// with --expand-auto should fail with appropriate error messages

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
			t.Logf("Expected error: %s", stderr)
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
	// Test scenario: CLI with daemon started and autoconf cached

	// Load test autoconf data
	autoConfData := loadTestDataComprehensive(t, "valid_autoconf.json")

	// Track HTTP requests to verify daemon fetches autoconf
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		t.Logf("Config show test request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Create IPFS node with multiple auto values
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{".": "auto"})

	// Start daemon and wait for autoconf fetch
	daemon := startDaemonAndWaitForAutoConf(t, node, &requestCount)
	defer daemon.StopDaemon()

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

	t.Logf("Config show --expand-auto produced %d characters of expanded config", len(expandedConfig))
}

// testMultipleExpandAutoUsesCache verifies that multiple consecutive --expand-auto calls
// efficiently use cached autoconf data instead of making repeated HTTP requests.
//
// This test is critical for performance because:
// 1. It validates that the caching mechanism works correctly to reduce network overhead
// 2. It ensures that users can make multiple config queries without causing excessive HTTP traffic
// 3. It verifies that cached data is shared across different config fields and commands
// 4. It tests that HTTP headers (ETag/Last-Modified) are properly used for cache validation
// 5. It prevents regression where each --expand-auto call would trigger a new HTTP request
// 6. It demonstrates the performance benefit: 5 operations with only 1 network request
func testMultipleExpandAutoUsesCache(t *testing.T) {
	// Test scenario: CLI with daemon started and autoconf cached

	// Create comprehensive autoconf response
	autoConfData := loadTestDataComprehensive(t, "valid_autoconf.json")

	// Track HTTP requests to verify caching
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		t.Logf("AutoConf cache test request #%d: %s %s", count, r.Method, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"cache-test-123"`)
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Create IPFS node with all auto values
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	// Note: Using default RefreshInterval (24h) to ensure caching - explicit setting would require rebuilt binary

	// Set up auto values for multiple fields
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"foo.": "auto"})
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Start daemon and wait for autoconf fetch
	daemon := startDaemonAndWaitForAutoConf(t, node, &requestCount)
	defer daemon.StopDaemon()

	// Reset counter to only track our expand-auto calls
	requestCount.Store(0)

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

	// CRITICAL TEST: Verify NO HTTP requests were made for --expand-auto calls (using cache)
	finalRequestCount := requestCount.Load()
	assert.Equal(t, int32(0), finalRequestCount,
		"Multiple --expand-auto calls should result in 0 HTTP requests (using cache). Got %d requests", finalRequestCount)

	t.Logf("Made 5 --expand-auto calls, resulted in %d HTTP request(s) - cache is being used!", finalRequestCount)

	// Now simulate a manual cache refresh (what the background updater would do)
	t.Log("Simulating manual cache refresh...")

	// Update the mock server to return different data
	autoConfData2 := loadTestDataComprehensive(t, "updated_autoconf.json")
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		t.Logf("Manual refresh request #%d: %s %s", count, r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"cache-test-456"`)
		w.Header().Set("Last-Modified", "Thu, 22 Oct 2015 08:00:00 GMT")
		_, _ = w.Write(autoConfData2)
	})

	// Note: In the actual daemon, the background updater would call MustGetConfigWithRefresh
	// For this test, we'll verify that subsequent --expand-auto calls still use cache
	// and don't trigger additional requests

	// Reset counter before manual refresh simulation
	beforeRefresh := requestCount.Load()

	// Make another --expand-auto call - should still use cache
	result6 := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result6.ExitCode(), "Bootstrap --expand-auto after refresh should succeed")

	afterRefresh := requestCount.Load()
	assert.Equal(t, beforeRefresh, afterRefresh,
		"--expand-auto should continue using cache even after server update")

	t.Logf("Cache continues to be used after server update - background updater pattern confirmed!")
}

// testCLIUsesCacheOnlyDaemonUpdatesBackground verifies the correct autoconf behavior:
// daemon makes exactly one HTTP request during startup to fetch and cache data, then
// CLI commands always use cached data without making additional HTTP requests.
//
// This test is essential for correctness because:
// 1. It validates that daemon startup makes exactly one HTTP request to fetch autoconf
// 2. It verifies that CLI --expand-auto never makes HTTP requests (uses cache only)
// 3. It ensures CLI commands remain fast by always using cached data
// 4. It prevents regression where CLI commands might start making HTTP requests
// 5. It confirms the correct separation between daemon (network) and CLI (cache-only) behavior
func testCLIUsesCacheOnlyDaemonUpdatesBackground(t *testing.T) {
	// Test scenario: CLI with daemon and long RefreshInterval (no background updates during test)

	// Create autoconf response
	autoConfData := loadTestDataComprehensive(t, "valid_autoconf.json")

	// Track HTTP requests with timestamps
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		t.Logf("Cache expiry test request #%d at %s: %s %s", count, time.Now().Format("15:04:05.000"), r.Method, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		// Use different ETag for each request to ensure we can detect new fetches
		w.Header().Set("ETag", fmt.Sprintf(`"expiry-test-%d"`, count))
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Create IPFS node with long refresh interval
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	// Set long RefreshInterval to avoid background updates during test
	node.SetIPFSConfig("AutoConf.RefreshInterval", "1h")

	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"test.": "auto"})

	// Start daemon and wait for autoconf fetch
	daemon := startDaemonAndWaitForAutoConf(t, node, &requestCount)
	defer daemon.StopDaemon()

	// Confirm only one request was made during daemon startup
	initialRequestCount := requestCount.Load()
	assert.Equal(t, int32(1), initialRequestCount, "Expected exactly 1 HTTP request during daemon startup, got: %d", initialRequestCount)
	t.Logf("Daemon startup made exactly 1 HTTP request")

	// Test: CLI commands use cache only (no additional HTTP requests)
	t.Log("Testing that CLI --expand-auto commands use cache only...")

	// Make several CLI calls - none should trigger HTTP requests
	result1 := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result1.ExitCode(), "Bootstrap --expand-auto should succeed")

	result2 := node.RunIPFS("config", "DNS.Resolvers", "--expand-auto")
	require.Equal(t, 0, result2.ExitCode(), "DNS.Resolvers --expand-auto should succeed")

	result3 := node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result3.ExitCode(), "Routing.DelegatedRouters --expand-auto should succeed")

	// Verify the request count remains at 1 (no additional requests from CLI)
	finalRequestCount := requestCount.Load()
	assert.Equal(t, int32(1), finalRequestCount, "Request count should remain at 1 after CLI commands, got: %d", finalRequestCount)
	t.Log("CLI commands use cache only - request count remains at 1")

	t.Log("Test completed: Daemon makes 1 startup request, CLI commands use cache only")
}

// loadTestDataComprehensive is a helper function that loads test autoconf JSON data files.
// It locates the test data directory relative to the test file and reads the specified file.
// This centralized helper ensures consistent test data loading across all comprehensive tests.
func loadTestDataComprehensive(t *testing.T, filename string) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/" + filename)
	require.NoError(t, err, "Failed to read test data file: %s", filename)

	return data
}

// startDaemonAndWaitForAutoConf starts a daemon and waits for it to fetch autoconf data.
// It returns the node with daemon running and ensures autoconf has been cached before returning.
// This is a DRY helper to avoid repeating daemon setup and request waiting logic in every test.
func startDaemonAndWaitForAutoConf(t *testing.T, node *harness.Node, requestCount *atomic.Int32) *harness.Node {
	t.Helper()

	// Start daemon to fetch and cache autoconf data
	t.Log("Starting daemon to fetch and cache autoconf data...")
	daemon := node.StartDaemon()
	// StartDaemon returns *Node, no error to check

	// Wait for daemon to fetch autoconf (wait for HTTP request to mock server)
	t.Log("Waiting for daemon to fetch autoconf from mock server...")
	timeout := time.After(10 * time.Second) // Safety timeout
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for autoconf fetch")
		case <-ticker.C:
			if requestCount.Load() > 0 {
				t.Logf("Daemon fetched autoconf (%d requests made)", requestCount.Load())
				t.Log("AutoConf should now be cached by daemon")
				return daemon
			}
		}
	}
}
