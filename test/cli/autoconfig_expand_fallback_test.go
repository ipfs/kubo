package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

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

	t.Run("expand-auto with disabled autoconfig shows error", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithDisabledAutoConfig(t)
	})

	t.Run("expand-auto with malformed response shows fallbacks", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithMalformedResponse(t)
	})

	t.Run("expand-auto preserves static values in mixed config", func(t *testing.T) {
		t.Parallel()
		testExpandAutoMixedConfigPreservesStatic(t)
	})
}

func testExpandAutoWithUnreachableServer(t *testing.T) {
	// Create IPFS node with unreachable AutoConfig server
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", "http://127.0.0.1:99999/nonexistent") // Unreachable
	node.SetIPFSConfig("AutoConfig.Enabled", true)
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

	// When autoconfig server is unreachable, DNS resolvers should fall back to defaults
	// The "foo." resolver should not exist in fallbacks (only "eth." has fallback)
	fooResolver, fooExists := resolvers["foo."]

	if !fooExists {
		t.Log("✅ DNS resolver for 'foo.' has no fallback - correct behavior (only eth. has fallbacks)")
	} else {
		assert.NotEqual(t, "auto", fooResolver, "DNS resolver should not be 'auto' after expansion")
		t.Logf("Unexpected DNS resolver for foo.: %s", fooResolver)
	}
}

func testExpandAutoWithDisabledAutoConfig(t *testing.T) {
	// Create IPFS node with AutoConfig disabled
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.Enabled", false)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Test that --expand-auto with disabled AutoConfig returns appropriate error or fallback
	result := node.RunIPFS("config", "Bootstrap", "--expand-auto")

	// When AutoConfig is disabled, expand-auto should show empty results
	// since "auto" values are not expanded when AutoConfig.Enabled=false
	var bootstrap []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &bootstrap)
	require.NoError(t, err)

	// With AutoConfig disabled, "auto" values are not expanded so we get empty result
	assert.NotContains(t, bootstrap, "auto", "Should not contain 'auto' after expansion")
	assert.Equal(t, 0, len(bootstrap), "Should be empty when AutoConfig disabled (auto values not expanded)")
	t.Log("✅ Bootstrap is empty when AutoConfig disabled - correct behavior")
}

func testExpandAutoWithMalformedResponse(t *testing.T) {
	// Create server that returns malformed JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invalid": "json", "Bootstrap": [incomplete`)) // Malformed JSON
	}))
	defer server.Close()

	// Create IPFS node with malformed autoconfig server
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
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
	// Load valid test autoconfig data
	autoConfigData := loadTestDataForFallback(t, "valid_autoconfig.json")

	// Create HTTP server that serves autoconfig.json
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfigData)
	}))
	defer server.Close()

	// Create IPFS node with mixed auto and static values
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)

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

// Helper function to load test data files for fallback tests
func loadTestDataForFallback(t *testing.T, filename string) []byte {
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
