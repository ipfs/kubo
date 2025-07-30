package autoconfig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestAutoConfigExpand(t *testing.T) {
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

	// Test with autoconfig server for --expand-auto functionality
	t.Run("config with --expand-auto expands auto values", func(t *testing.T) {
		// Load test autoconfig data
		autoConfigData := loadTestDataExpand(t, "valid_autoconfig.json")

		// Create HTTP server that serves autoconfig.json
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(autoConfigData)
		}))
		defer server.Close()

		// Configure autoconfig for the node
		node.SetIPFSConfig("AutoConfig.URL", server.URL)
		node.SetIPFSConfig("AutoConfig.Enabled", true)

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

// Helper function to load test data files
func loadTestDataExpand(t *testing.T, filename string) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/" + filename)
	require.NoError(t, err, "Failed to read test data file: %s", filename)

	return data
}
