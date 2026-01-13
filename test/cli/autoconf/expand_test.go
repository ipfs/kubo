package autoconf

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoConfExpand(t *testing.T) {
	t.Parallel()

	t.Run("config commands show auto values", func(t *testing.T) {
		t.Parallel()
		testConfigCommandsShowAutoValues(t)
	})

	t.Run("mixed configuration preserves both auto and static", func(t *testing.T) {
		t.Parallel()
		testMixedConfigurationPreserved(t)
	})

	t.Run("config replace preserves auto values", func(t *testing.T) {
		t.Parallel()
		testConfigReplacePreservesAuto(t)
	})

	t.Run("expand-auto filters unsupported URL paths with delegated routing", func(t *testing.T) {
		t.Parallel()
		testExpandAutoFiltersUnsupportedPathsDelegated(t)
	})

	t.Run("expand-auto with auto routing uses NewRoutingSystem", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithAutoRouting(t)
	})

	t.Run("expand-auto with auto routing shows AminoDHT native vs IPNI delegated", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithMixedSystems(t)
	})

	t.Run("expand-auto filters paths with NewRoutingSystem and auto routing", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithFiltering(t)
	})

	t.Run("expand-auto falls back to defaults without cache (delegated)", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithoutCacheDelegated(t)
	})

	t.Run("expand-auto with auto routing without cache", func(t *testing.T) {
		t.Parallel()
		testExpandAutoWithoutCacheAuto(t)
	})
}

func testConfigCommandsShowAutoValues(t *testing.T) {
	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Set all fields to "auto"
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"foo.": "auto"})
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Test individual field queries
	t.Run("Bootstrap shows auto", func(t *testing.T) {
		result := node.RunIPFS("config", "Bootstrap")
		require.Equal(t, 0, result.ExitCode())

		var bootstrap []string
		err := json.Unmarshal([]byte(result.Stdout.String()), &bootstrap)
		require.NoError(t, err)
		assert.Equal(t, []string{"auto"}, bootstrap)
	})

	t.Run("DNS.Resolvers shows auto", func(t *testing.T) {
		result := node.RunIPFS("config", "DNS.Resolvers")
		require.Equal(t, 0, result.ExitCode())

		var resolvers map[string]string
		err := json.Unmarshal([]byte(result.Stdout.String()), &resolvers)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"foo.": "auto"}, resolvers)
	})

	t.Run("Routing.DelegatedRouters shows auto", func(t *testing.T) {
		result := node.RunIPFS("config", "Routing.DelegatedRouters")
		require.Equal(t, 0, result.ExitCode())

		var routers []string
		err := json.Unmarshal([]byte(result.Stdout.String()), &routers)
		require.NoError(t, err)
		assert.Equal(t, []string{"auto"}, routers)
	})

	t.Run("Ipns.DelegatedPublishers shows auto", func(t *testing.T) {
		result := node.RunIPFS("config", "Ipns.DelegatedPublishers")
		require.Equal(t, 0, result.ExitCode())

		var publishers []string
		err := json.Unmarshal([]byte(result.Stdout.String()), &publishers)
		require.NoError(t, err)
		assert.Equal(t, []string{"auto"}, publishers)
	})

	t.Run("config show contains all auto values", func(t *testing.T) {
		result := node.RunIPFS("config", "show")
		require.Equal(t, 0, result.ExitCode())

		output := result.Stdout.String()

		// Check that auto values are present in the full config
		assert.Contains(t, output, `"Bootstrap": [
    "auto"
  ]`, "Bootstrap should contain auto")

		assert.Contains(t, output, `"DNS": {
    "Resolvers": {
      "foo.": "auto"
    }
  }`, "DNS.Resolvers should contain auto")

		assert.Contains(t, output, `"DelegatedRouters": [
      "auto"
    ]`, "Routing.DelegatedRouters should contain auto")

		assert.Contains(t, output, `"DelegatedPublishers": [
      "auto"
    ]`, "Ipns.DelegatedPublishers should contain auto")
	})

	// Test with autoconf server for --expand-auto functionality
	t.Run("config with --expand-auto expands auto values", func(t *testing.T) {
		// Load test autoconf data
		autoConfData := loadTestDataExpand(t, "valid_autoconf.json")

		// Create HTTP server that serves autoconf.json
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(autoConfData)
		}))
		defer server.Close()

		// Configure autoconf for the node
		node.SetIPFSConfig("AutoConf.URL", server.URL)
		node.SetIPFSConfig("AutoConf.Enabled", true)

		// Test Bootstrap field expansion
		result := node.RunIPFS("config", "Bootstrap", "--expand-auto")
		require.Equal(t, 0, result.ExitCode(), "config Bootstrap --expand-auto should succeed")

		var expandedBootstrap []string
		err := json.Unmarshal([]byte(result.Stdout.String()), &expandedBootstrap)
		require.NoError(t, err)
		assert.NotContains(t, expandedBootstrap, "auto", "Expanded bootstrap should not contain 'auto'")
		assert.Greater(t, len(expandedBootstrap), 0, "Expanded bootstrap should contain expanded peers")

		// Test DNS.Resolvers field expansion
		result = node.RunIPFS("config", "DNS.Resolvers", "--expand-auto")
		require.Equal(t, 0, result.ExitCode(), "config DNS.Resolvers --expand-auto should succeed")

		var expandedResolvers map[string]string
		err = json.Unmarshal([]byte(result.Stdout.String()), &expandedResolvers)
		require.NoError(t, err)
		assert.NotEqual(t, "auto", expandedResolvers["foo."], "Expanded DNS resolver should not be 'auto'")

		// Test Routing.DelegatedRouters field expansion
		result = node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
		require.Equal(t, 0, result.ExitCode(), "config Routing.DelegatedRouters --expand-auto should succeed")

		var expandedRouters []string
		err = json.Unmarshal([]byte(result.Stdout.String()), &expandedRouters)
		require.NoError(t, err)
		assert.NotContains(t, expandedRouters, "auto", "Expanded routers should not contain 'auto'")

		// Test Ipns.DelegatedPublishers field expansion
		result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
		require.Equal(t, 0, result.ExitCode(), "config Ipns.DelegatedPublishers --expand-auto should succeed")

		var expandedPublishers []string
		err = json.Unmarshal([]byte(result.Stdout.String()), &expandedPublishers)
		require.NoError(t, err)
		assert.NotContains(t, expandedPublishers, "auto", "Expanded publishers should not contain 'auto'")

		// Test config show --expand-auto (full config expansion)
		result = node.RunIPFS("config", "show", "--expand-auto")
		require.Equal(t, 0, result.ExitCode(), "config show --expand-auto should succeed")

		expandedOutput := result.Stdout.String()
		t.Logf("Expanded config output contains: %d characters", len(expandedOutput))

		// Verify that auto values are expanded in the full config
		assert.NotContains(t, expandedOutput, `"auto"`, "Expanded config should not contain literal 'auto' values")
		assert.Contains(t, expandedOutput, `"Bootstrap"`, "Expanded config should contain Bootstrap section")
		assert.Contains(t, expandedOutput, `"DNS"`, "Expanded config should contain DNS section")
	})
}

func testMixedConfigurationPreserved(t *testing.T) {
	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Set mixed configuration
	node.SetIPFSConfig("Bootstrap", []string{
		"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTest",
		"auto",
		"/ip4/127.0.0.2/tcp/4001/p2p/12D3KooWTest2",
	})

	node.SetIPFSConfig("DNS.Resolvers", map[string]string{
		"eth.": "https://eth.resolver",
		"foo.": "auto",
		"bar.": "https://bar.resolver",
	})

	node.SetIPFSConfig("Routing.DelegatedRouters", []string{
		"https://static.router",
		"auto",
	})

	// Verify Bootstrap preserves order and mixes auto with static
	result := node.RunIPFS("config", "Bootstrap")
	require.Equal(t, 0, result.ExitCode())

	var bootstrap []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &bootstrap)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTest",
		"auto",
		"/ip4/127.0.0.2/tcp/4001/p2p/12D3KooWTest2",
	}, bootstrap)

	// Verify DNS.Resolvers preserves both auto and static
	result = node.RunIPFS("config", "DNS.Resolvers")
	require.Equal(t, 0, result.ExitCode())

	var resolvers map[string]string
	err = json.Unmarshal([]byte(result.Stdout.String()), &resolvers)
	require.NoError(t, err)
	assert.Equal(t, "https://eth.resolver", resolvers["eth."])
	assert.Equal(t, "auto", resolvers["foo."])
	assert.Equal(t, "https://bar.resolver", resolvers["bar."])

	// Verify Routing.DelegatedRouters preserves order
	result = node.RunIPFS("config", "Routing.DelegatedRouters")
	require.Equal(t, 0, result.ExitCode())

	var routers []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &routers)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"https://static.router",
		"auto",
	}, routers)
}

func testConfigReplacePreservesAuto(t *testing.T) {
	// Create IPFS node
	h := harness.NewT(t)
	node := h.NewNode().Init("--profile=test")

	// Set initial auto values
	node.SetIPFSConfig("Bootstrap", []string{"auto"})
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"foo.": "auto"})

	// Export current config
	result := node.RunIPFS("config", "show")
	require.Equal(t, 0, result.ExitCode())
	originalConfig := result.Stdout.String()

	// Verify auto values are in the exported config
	assert.Contains(t, originalConfig, `"Bootstrap": [
    "auto"
  ]`)
	assert.Contains(t, originalConfig, `"foo.": "auto"`)

	// Modify the config string to add a new field but preserve auto values
	var configMap map[string]interface{}
	err := json.Unmarshal([]byte(originalConfig), &configMap)
	require.NoError(t, err)

	// Add a new field
	configMap["NewTestField"] = "test-value"

	// Marshal back to JSON
	modifiedConfig, err := json.MarshalIndent(configMap, "", "  ")
	require.NoError(t, err)

	// Write config to file and replace
	configFile := h.WriteToTemp(string(modifiedConfig))
	replaceResult := node.RunIPFS("config", "replace", configFile)
	if replaceResult.ExitCode() != 0 {
		t.Logf("Config replace failed: stdout=%s, stderr=%s", replaceResult.Stdout.String(), replaceResult.Stderr.String())
	}
	require.Equal(t, 0, replaceResult.ExitCode())

	// Verify auto values are still present after replace
	result = node.RunIPFS("config", "Bootstrap")
	require.Equal(t, 0, result.ExitCode())

	var bootstrap []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &bootstrap)
	require.NoError(t, err)
	assert.Equal(t, []string{"auto"}, bootstrap, "Bootstrap should still contain auto after config replace")

	// Verify DNS resolver config is preserved after replace
	result = node.RunIPFS("config", "DNS.Resolvers")
	require.Equal(t, 0, result.ExitCode())

	var resolvers map[string]string
	err = json.Unmarshal([]byte(result.Stdout.String()), &resolvers)
	require.NoError(t, err)
	assert.Equal(t, "auto", resolvers["foo."], "DNS resolver for foo. should still be auto after config replace")
}

func testExpandAutoFiltersUnsupportedPathsDelegated(t *testing.T) {
	// Test scenario: CLI with daemon started and autoconf cached using delegated routing
	// This tests the production scenario where delegated routing is enabled and
	// daemon has fetched and cached autoconf data, and CLI commands read from that cache

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Configure delegated routing to use autoconf URLs
	node.SetIPFSConfig("Routing.Type", "delegated")
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})
	// Disable content providing when using delegated routing
	node.SetIPFSConfig("Provide.Enabled", false)
	node.SetIPFSConfig("Provide.DHT.Interval", "0")

	// Load test autoconf data with unsupported paths
	autoConfData := loadTestDataExpand(t, "autoconf_with_unsupported_paths.json")

	// Create HTTP server that serves autoconf.json with unsupported paths
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Configure autoconf for the node
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)

	// Verify the autoconf URL is set correctly
	result := node.RunIPFS("config", "AutoConf.URL")
	require.Equal(t, 0, result.ExitCode(), "config AutoConf.URL should succeed")
	t.Logf("AutoConf URL is set to: %s", result.Stdout.String())
	assert.Contains(t, result.Stdout.String(), "127.0.0.1", "AutoConf URL should contain the test server address")

	// Start daemon to fetch and cache autoconf data
	t.Log("Starting daemon to fetch and cache autoconf data...")
	daemon := node.StartDaemon()
	defer daemon.StopDaemon()

	// Wait for autoconf fetch (use autoconf default timeout + buffer)
	time.Sleep(6 * time.Second) // defaultTimeout is 5s, add 1s buffer
	t.Log("AutoConf should now be cached by daemon")

	// Test Routing.DelegatedRouters field expansion filters unsupported paths
	result = node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Routing.DelegatedRouters --expand-auto should succeed")

	var expandedRouters []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &expandedRouters)
	require.NoError(t, err)

	// After cache prewarming, should get URLs from autoconf that have supported paths
	assert.Contains(t, expandedRouters, "https://supported.example.com/routing/v1/providers", "Should contain supported provider URL")
	assert.Contains(t, expandedRouters, "https://supported.example.com/routing/v1/peers", "Should contain supported peers URL")
	assert.Contains(t, expandedRouters, "https://mixed.example.com/routing/v1/providers", "Should contain mixed provider URL")
	assert.Contains(t, expandedRouters, "https://mixed.example.com/routing/v1/peers", "Should contain mixed peers URL")

	// Verify unsupported URLs from autoconf are filtered out (not in result)
	assert.NotContains(t, expandedRouters, "https://unsupported.example.com/example/v0/read", "Should filter out unsupported path /example/v0/read")
	assert.NotContains(t, expandedRouters, "https://unsupported.example.com/api/v1/custom", "Should filter out unsupported path /api/v1/custom")
	assert.NotContains(t, expandedRouters, "https://mixed.example.com/unsupported/path", "Should filter out unsupported path /unsupported/path")

	t.Logf("Filtered routers: %v", expandedRouters)

	// Test Ipns.DelegatedPublishers field expansion filters unsupported paths
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Ipns.DelegatedPublishers --expand-auto should succeed")

	var expandedPublishers []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedPublishers)
	require.NoError(t, err)

	// After cache prewarming, should get URLs from autoconf that have supported paths
	assert.Contains(t, expandedPublishers, "https://supported.example.com/routing/v1/ipns", "Should contain supported IPNS URL")
	assert.Contains(t, expandedPublishers, "https://mixed.example.com/routing/v1/ipns", "Should contain mixed IPNS URL")

	// Verify unsupported URLs from autoconf are filtered out (not in result)
	assert.NotContains(t, expandedPublishers, "https://unsupported.example.com/example/v0/write", "Should filter out unsupported write path")

	t.Logf("Filtered publishers: %v", expandedPublishers)
}

func testExpandAutoWithoutCacheDelegated(t *testing.T) {
	// Test scenario: CLI without daemon ever starting (no cached autoconf) using delegated routing
	// This tests the fallback scenario where delegated routing is configured but CLI commands
	// cannot read from cache and must fall back to hardcoded defaults

	// Create IPFS node but DO NOT start daemon
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Configure delegated routing to use autoconf URLs (but no daemon to fetch them)
	node.SetIPFSConfig("Routing.Type", "delegated")
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})
	// Disable content providing when using delegated routing
	node.SetIPFSConfig("Provide.Enabled", false)
	node.SetIPFSConfig("Provide.DHT.Interval", "0")

	// Load test autoconf data with unsupported paths (this won't be used since no daemon)
	autoConfData := loadTestDataExpand(t, "autoconf_with_unsupported_paths.json")

	// Create HTTP server that serves autoconf.json with unsupported paths
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Configure autoconf for the node (but daemon never starts to fetch it)
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)

	// Test Routing.DelegatedRouters field expansion without cached autoconf
	result := node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Routing.DelegatedRouters --expand-auto should succeed")

	var expandedRouters []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &expandedRouters)
	require.NoError(t, err)

	// Without cached autoconf, should get fallback URLs from GetMainnetFallbackConfig()
	// NOTE: These values may change if autoconf library updates GetMainnetFallbackConfig()
	assert.Contains(t, expandedRouters, "https://cid.contact/routing/v1/providers", "Should contain fallback provider URL from GetMainnetFallbackConfig()")

	t.Logf("Fallback routers (no cache): %v", expandedRouters)

	// Test Ipns.DelegatedPublishers field expansion without cached autoconf
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Ipns.DelegatedPublishers --expand-auto should succeed")

	var expandedPublishers []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedPublishers)
	require.NoError(t, err)

	// Without cached autoconf, should get fallback IPNS publishers from GetMainnetFallbackConfig()
	// NOTE: These values may change if autoconf library updates GetMainnetFallbackConfig()
	assert.Contains(t, expandedPublishers, "https://delegated-ipfs.dev/routing/v1/ipns", "Should contain fallback IPNS URL from GetMainnetFallbackConfig()")

	t.Logf("Fallback publishers (no cache): %v", expandedPublishers)
}

func testExpandAutoWithAutoRouting(t *testing.T) {
	// Test scenario: CLI with daemon started using auto routing with NewRoutingSystem
	// This tests that non-native systems (NewRoutingSystem) ARE delegated even with auto routing
	// Only native systems like AminoDHT are handled internally with auto routing

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Configure auto routing with non-native system
	node.SetIPFSConfig("Routing.Type", "auto")
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Load test autoconf data with NewRoutingSystem (non-native, will be delegated)
	autoConfData := loadTestDataExpand(t, "autoconf_new_routing_system.json")

	// Create HTTP server that serves autoconf.json
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Configure autoconf for the node
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)

	// Start daemon to fetch and cache autoconf data
	t.Log("Starting daemon to fetch and cache autoconf data...")
	daemon := node.StartDaemon()
	defer daemon.StopDaemon()

	// Wait for autoconf fetch (use autoconf default timeout + buffer)
	time.Sleep(6 * time.Second) // defaultTimeout is 5s, add 1s buffer
	t.Log("AutoConf should now be cached by daemon")

	// Test Routing.DelegatedRouters field expansion with auto routing
	result := node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Routing.DelegatedRouters --expand-auto should succeed")

	var expandedRouters []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &expandedRouters)
	require.NoError(t, err)

	// With auto routing and NewRoutingSystem (non-native), delegated endpoints should be populated
	assert.Contains(t, expandedRouters, "https://new-routing.example.com/routing/v1/providers", "Should contain NewRoutingSystem provider URL")
	assert.Contains(t, expandedRouters, "https://new-routing.example.com/routing/v1/peers", "Should contain NewRoutingSystem peers URL")

	t.Logf("Auto routing routers (NewRoutingSystem delegated): %v", expandedRouters)

	// Test Ipns.DelegatedPublishers field expansion with auto routing
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Ipns.DelegatedPublishers --expand-auto should succeed")

	var expandedPublishers []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedPublishers)
	require.NoError(t, err)

	// With auto routing and NewRoutingSystem (non-native), delegated publishers should be populated
	assert.Contains(t, expandedPublishers, "https://new-routing.example.com/routing/v1/ipns", "Should contain NewRoutingSystem IPNS URL")

	t.Logf("Auto routing publishers (NewRoutingSystem delegated): %v", expandedPublishers)
}

func testExpandAutoWithMixedSystems(t *testing.T) {
	// Test scenario: Auto routing with both AminoDHT (native) and IPNI (delegated) systems
	// This explicitly confirms that AminoDHT is NOT delegated but IPNI at cid.contact IS delegated

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Configure auto routing
	node.SetIPFSConfig("Routing.Type", "auto")
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Load test autoconf data with both AminoDHT and IPNI systems
	autoConfData := loadTestDataExpand(t, "autoconf_amino_and_ipni.json")

	// Create HTTP server that serves autoconf.json
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Configure autoconf for the node
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)

	// Start daemon to fetch and cache autoconf data
	t.Log("Starting daemon to fetch and cache autoconf data...")
	daemon := node.StartDaemon()
	defer daemon.StopDaemon()

	// Wait for autoconf fetch (use autoconf default timeout + buffer)
	time.Sleep(6 * time.Second) // defaultTimeout is 5s, add 1s buffer
	t.Log("AutoConf should now be cached by daemon")

	// Test Routing.DelegatedRouters field expansion
	result := node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Routing.DelegatedRouters --expand-auto should succeed")

	var expandedRouters []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &expandedRouters)
	require.NoError(t, err)

	// With auto routing: AminoDHT (native) should NOT be delegated, IPNI should be delegated
	assert.Contains(t, expandedRouters, "https://cid.contact/routing/v1/providers", "Should contain IPNI provider URL (delegated)")
	assert.NotContains(t, expandedRouters, "https://amino-dht.example.com", "Should NOT contain AminoDHT URLs (native)")

	t.Logf("Mixed systems routers (IPNI delegated, AminoDHT native): %v", expandedRouters)

	// Test Ipns.DelegatedPublishers field expansion
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Ipns.DelegatedPublishers --expand-auto should succeed")

	var expandedPublishers []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedPublishers)
	require.NoError(t, err)

	// IPNI system doesn't have write endpoints, so publishers should be empty
	// (or contain other systems if they have write endpoints)
	t.Logf("Mixed systems publishers (IPNI has no write endpoints): %v", expandedPublishers)
}

func testExpandAutoWithFiltering(t *testing.T) {
	// Test scenario: Auto routing with NewRoutingSystem and path filtering
	// This tests that path filtering works for delegated systems even with auto routing

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Configure auto routing
	node.SetIPFSConfig("Routing.Type", "auto")
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Load test autoconf data with NewRoutingSystem and mixed valid/invalid paths
	autoConfData := loadTestDataExpand(t, "autoconf_new_routing_with_filtering.json")

	// Create HTTP server that serves autoconf.json
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Configure autoconf for the node
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)

	// Start daemon to fetch and cache autoconf data
	t.Log("Starting daemon to fetch and cache autoconf data...")
	daemon := node.StartDaemon()
	defer daemon.StopDaemon()

	// Wait for autoconf fetch (use autoconf default timeout + buffer)
	time.Sleep(6 * time.Second) // defaultTimeout is 5s, add 1s buffer
	t.Log("AutoConf should now be cached by daemon")

	// Test Routing.DelegatedRouters field expansion with filtering
	result := node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Routing.DelegatedRouters --expand-auto should succeed")

	var expandedRouters []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &expandedRouters)
	require.NoError(t, err)

	// Should contain supported paths from NewRoutingSystem
	assert.Contains(t, expandedRouters, "https://supported-new.example.com/routing/v1/providers", "Should contain supported provider URL")
	assert.Contains(t, expandedRouters, "https://supported-new.example.com/routing/v1/peers", "Should contain supported peers URL")
	assert.Contains(t, expandedRouters, "https://mixed-new.example.com/routing/v1/providers", "Should contain mixed provider URL")
	assert.Contains(t, expandedRouters, "https://mixed-new.example.com/routing/v1/peers", "Should contain mixed peers URL")

	// Should NOT contain unsupported paths
	assert.NotContains(t, expandedRouters, "https://unsupported-new.example.com/custom/v0/read", "Should filter out unsupported path")
	assert.NotContains(t, expandedRouters, "https://unsupported-new.example.com/api/v1/nonstandard", "Should filter out unsupported path")
	assert.NotContains(t, expandedRouters, "https://mixed-new.example.com/invalid/path", "Should filter out invalid path from mixed endpoint")

	t.Logf("Filtered routers (NewRoutingSystem with auto routing): %v", expandedRouters)

	// Test Ipns.DelegatedPublishers field expansion with filtering
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Ipns.DelegatedPublishers --expand-auto should succeed")

	var expandedPublishers []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedPublishers)
	require.NoError(t, err)

	// Should contain supported IPNS paths
	assert.Contains(t, expandedPublishers, "https://supported-new.example.com/routing/v1/ipns", "Should contain supported IPNS URL")
	assert.Contains(t, expandedPublishers, "https://mixed-new.example.com/routing/v1/ipns", "Should contain mixed IPNS URL")

	// Should NOT contain unsupported write paths
	assert.NotContains(t, expandedPublishers, "https://unsupported-new.example.com/custom/v0/write", "Should filter out unsupported write path")

	t.Logf("Filtered publishers (NewRoutingSystem with auto routing): %v", expandedPublishers)
}

func testExpandAutoWithoutCacheAuto(t *testing.T) {
	// Test scenario: CLI without daemon ever starting using auto routing (default)
	// This tests the fallback scenario where auto routing is used but doesn't populate delegated config fields

	// Create IPFS node but DO NOT start daemon
	node := harness.NewT(t).NewNode().Init("--profile=test")

	// Configure auto routing - delegated fields are set to "auto" but won't be populated
	// because auto routing uses different internal mechanisms
	node.SetIPFSConfig("Routing.Type", "auto")
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Load test autoconf data (this won't be used since no daemon and auto routing doesn't use these fields)
	autoConfData := loadTestDataExpand(t, "autoconf_with_unsupported_paths.json")

	// Create HTTP server (won't be contacted since no daemon)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(autoConfData)
	}))
	defer server.Close()

	// Configure autoconf for the node (but daemon never starts to fetch it)
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)

	// Test Routing.DelegatedRouters field expansion without cached autoconf
	result := node.RunIPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Routing.DelegatedRouters --expand-auto should succeed")

	var expandedRouters []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &expandedRouters)
	require.NoError(t, err)

	// With auto routing, some fallback URLs are still populated from GetMainnetFallbackConfig()
	// NOTE: These values may change if autoconf library updates GetMainnetFallbackConfig()
	assert.Contains(t, expandedRouters, "https://cid.contact/routing/v1/providers", "Should contain fallback provider URL from GetMainnetFallbackConfig()")

	t.Logf("Auto routing fallback routers (with fallbacks): %v", expandedRouters)

	// Test Ipns.DelegatedPublishers field expansion without cached autoconf
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	require.Equal(t, 0, result.ExitCode(), "config Ipns.DelegatedPublishers --expand-auto should succeed")

	var expandedPublishers []string
	err = json.Unmarshal([]byte(result.Stdout.String()), &expandedPublishers)
	require.NoError(t, err)

	// With auto routing, delegated publishers may be empty for fallback scenario
	// This can vary based on which systems have write endpoints in the fallback config
	t.Logf("Auto routing fallback publishers: %v", expandedPublishers)
}

// Helper function to load test data files
func loadTestDataExpand(t *testing.T, filename string) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/" + filename)
	require.NoError(t, err, "Failed to read test data file: %s", filename)

	return data
}
