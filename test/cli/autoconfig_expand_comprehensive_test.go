package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
}

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

// Helper function to load test data files for comprehensive tests
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
