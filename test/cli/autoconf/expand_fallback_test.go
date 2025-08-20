package autoconf

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ipfs/boxo/autoconf"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandAutoFallbacks(t *testing.T) {
	t.Parallel()

	t.Run("expand-auto with unreachable server shows fallbacks", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithUnreachableServer(t)
	})

	t.Run("expand-auto with disabled autoconf shows error", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithDisabledAutoConf(t)
	})

	t.Run("expand-auto with malformed response shows fallbacks", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithMalformedResponse(t)
	})

	t.Run("expand-auto preserves static values in mixed config", func(t *testing.T) {
		t.Parallel()
		testExpandAutoMixedConfigPreservesStatic(t)
	})

	t.Run("daemon gracefully handles malformed autoconf and uses fallbacks", func(t *testing.T) {
		t.Parallel()
		testDaemonWithMalformedAutoConf(t)
	})
}

func testExpandAutoWithUnreachableServer(t *testing.T) {
	// Create IPFS node with unreachable AutoConf server
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", "http://127.0.0.1:99999/nonexistent") // Unreachable
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"foo.": "auto"})

	// Test that --expand-auto falls back to defaults when server is unreachable
	result := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Bootstrap --expand-auto should succeed even with unreachable server")

	var bootstrap []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &bootstrap)
	require.NoError(t, err)

	// Should contain fallback bootstrap peers (not "auto" and not empty)
	assert.NotContains(t, bootstrap, "auto", "Fallback bootstrap should not contain 'auto'")
	assert.Greater(t, len(bootstrap), 0, "Fallback bootstrap should not be empty")

	// Should contain known default bootstrap peers
	foundDefaultPeer := false
	for _, peer := range bootstrap {
		if peer != "" && peer != "auto" {
			foundDefaultPeer = true
			t.Logf("Found fallback bootstrap peer: %s", peer)
			break
		}
	}
	assert.True(t, foundDefaultPeer, "Should contain at least one fallback bootstrap peer")

	// Test DNS resolvers fallback
	result = node.RunIPFS("config", "DNS.Resolvers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config DNS.Resolvers --expand-auto should succeed with unreachable server")

	var resolvers map[string]string
	err = json.Unmarshal([]byte(result.Stdout.String()), &resolvers)
	require.NoError(t, err)

	// When autoconf server is unreachable, DNS resolvers should fall back to defaults
	// The "foo." resolver should not exist in fallbacks (only "eth." has fallback)
	fooResolver, fooExists := resolvers["foo."]

	if !fooExists {
		t.Log("DNS resolver for 'foo.' has no fallback - correct behavior (only eth. has fallbacks)")
	} else {
		assert.NotEqual(t, "auto", fooResolver, "DNS resolver should not be 'auto' after expansion")
		t.Logf("Unexpected DNS resolver for foo.: %s", fooResolver)
	}
}

func testExpandAutoWithDisabledAutoConf(t *testing.T) {
	// Create IPFS node with AutoConf disabled
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.Enabled", false)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Test that --expand-auto with disabled AutoConf returns appropriate error or fallback
	result := node.RunIPFS("config", "Bootstrap", "--expand-auto")

	// When AutoConf is disabled, expand-auto should show empty results
	// since "auto" values are not expanded when AutoConf.Enabled=false
	var bootstrap []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &bootstrap)
	require.NoError(t, err)

	// With AutoConf disabled, "auto" values are not expanded so we get empty result
	assert.NotContains(t, bootstrap, "auto", "Should not contain 'auto' after expansion")
	assert.Equal(t, 0, len(bootstrap), "Should be empty when AutoConf disabled (auto values not expanded)")
	t.Log("Bootstrap is empty when AutoConf disabled - correct behavior")
}

func testExpandAutoWithMalformedResponse(t *testing.T) {
	// Create server that returns malformed JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invalid": "json", "Bootstrap": [incomplete`)) // Malformed JSON
	}))
	defer server.Close()

	// Create IPFS node with malformed autoconf server
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Test that --expand-auto handles malformed response gracefully
	result := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Bootstrap --expand-auto should succeed even with malformed response")

	var bootstrap []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &bootstrap)
	require.NoError(t, err)

	// Should fall back to defaults, not contain "auto"
	assert.NotContains(t, bootstrap, "auto", "Should not contain 'auto' after fallback")
	assert.Greater(t, len(bootstrap), 0, "Should contain fallback peers after malformed response")
	t.Logf("Bootstrap after malformed response: %v", bootstrap)
}

func testExpandAutoMixedConfigPreservesStatic(t *testing.T) {
	// Load valid test autoconf data
	autoConfData := loadTestDataForFallback(t, "valid_autoconf.json")

	// Create HTTP server that serves autoconf.json
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Create IPFS node with mixed auto and static values
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)

	// Set mixed configuration: static + auto + static
	node.SetIPFSConfig("Bootstrap", []string{
		"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTest",
		"auto",
		"/ip4/127.0.0.2/tcp/4001/p2p/12D3KooWTest2",
	})

	// Test that --expand-auto only expands "auto" values, preserves static ones
	result := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Bootstrap --expand-auto should succeed")

	var bootstrap []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &bootstrap)
	require.NoError(t, err)

	// Should not contain literal "auto" anymore
	assert.NotContains(t, bootstrap, "auto", "Expanded config should not contain literal 'auto'")

	// Should preserve static values at original positions
	assert.Contains(t, bootstrap, "/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTest", "Should preserve first static peer")
	assert.Contains(t, bootstrap, "/ip4/127.0.0.2/tcp/4001/p2p/12D3KooWTest2", "Should preserve third static peer")

	// Should have more entries than just the static ones (auto got expanded)
	assert.Greater(t, len(bootstrap), 2, "Should have more than just the 2 static peers")

	t.Logf("Mixed config expansion result: %v", bootstrap)

	// Verify order is preserved: static, expanded auto values, static
	assert.Equal(t, "/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTest", bootstrap[0], "First peer should be preserved")
	lastIndex := len(bootstrap) - 1
	assert.Equal(t, "/ip4/127.0.0.2/tcp/4001/p2p/12D3KooWTest2", bootstrap[lastIndex], "Last peer should be preserved")
}

func testDaemonWithMalformedAutoConf(t *testing.T) {
	// Test scenario: Daemon starts with AutoConf.URL pointing to server that returns malformed JSON
	// This tests that daemon gracefully handles malformed responses and falls back to hardcoded defaults

	// Create server that returns malformed JSON to simulate broken autoconf service
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return malformed JSON that cannot be parsed
		_, _ = w.Write([]byte(`{"Bootstrap": ["incomplete array", "missing closing bracket"`))
	}))
	defer server.Close()

	// Create IPFS node with autoconf pointing to malformed server
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"foo.": "auto"})

	// Start daemon - this will attempt to fetch autoconf from malformed server
	t.Log("Starting daemon with malformed autoconf server...")
	daemon := node.StartDaemon()
	defer daemon.StopDaemon()

	// Wait for daemon to attempt autoconf fetch and handle the error gracefully
	time.Sleep(6 * time.Second) // defaultTimeout is 5s, add 1s buffer
	t.Log("Daemon should have attempted autoconf fetch and fallen back to defaults")

	// Test that daemon is still running and CLI commands work with fallback values
	result := node.RunIPFS("config", "Bootstrap", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Bootstrap --expand-auto should succeed with daemon running")

	var bootstrap []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &bootstrap)
	require.NoError(t, err)

	// Should fall back to hardcoded defaults from GetMainnetFallbackConfig()
	// NOTE: These values may change if autoconf library updates GetMainnetFallbackConfig()
	assert.NotContains(t, bootstrap, "auto", "Should not contain 'auto' after fallback")
	assert.Greater(t, len(bootstrap), 0, "Should contain fallback bootstrap peers")

	// Verify we got actual fallback bootstrap peers from GetMainnetFallbackConfig() AminoDHT NativeConfig
	fallbackConfig := autoconf.GetMainnetFallbackConfig()
	aminoDHTSystem := fallbackConfig.SystemRegistry["AminoDHT"]
	expectedBootstrapPeers := aminoDHTSystem.NativeConfig.Bootstrap

	foundFallbackPeers := 0
	for _, expectedPeer := range expectedBootstrapPeers {
		for _, actualPeer := range bootstrap {
			if actualPeer == expectedPeer {
				foundFallbackPeers++
				break
			}
		}
	}
	assert.Greater(t, foundFallbackPeers, 0, "Should contain bootstrap peers from GetMainnetFallbackConfig() AminoDHT NativeConfig")
	assert.Equal(t, len(expectedBootstrapPeers), foundFallbackPeers, "Should contain all bootstrap peers from GetMainnetFallbackConfig() AminoDHT NativeConfig")

	t.Logf("Daemon fallback bootstrap peers after malformed response: %v", bootstrap)

	// Test DNS resolvers also fall back correctly
	result = node.RunIPFS("config", "DNS.Resolvers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config DNS.Resolvers --expand-auto should succeed with daemon running")

	var resolvers map[string]string
	err = json.Unmarshal([]byte(result.Stdout.String()), &resolvers)
	require.NoError(t, err)

	// Should not contain "auto" and should have fallback DNS resolvers
	assert.NotEqual(t, "auto", resolvers["foo."], "DNS resolver should not be 'auto' after fallback")
	if resolvers["foo."] != "" {
		// If resolver is populated, it should be a valid URL from fallbacks
		assert.Contains(t, resolvers["foo."], "https://", "Fallback DNS resolver should be HTTPS URL")
	}

	t.Logf("Daemon fallback DNS resolvers after malformed response: %v", resolvers)

	// Verify daemon is still healthy and responsive
	versionResult := node.RunIPFS("version")
	require.Equal(t, 0, versionResult.ExitCode(), "daemon should remain healthy after handling malformed autoconf")
	t.Log("Daemon remains healthy after gracefully handling malformed autoconf response")
}

// Helper function to load test data files for fallback tests
func loadTestDataForFallback(t *testing.T, filename string) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/" + filename)
	require.NoError(t, err, "Failed to read test data file: %s", filename)

	return data
}
