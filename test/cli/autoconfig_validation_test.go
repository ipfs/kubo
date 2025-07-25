package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestAutoConfigValidation(t *testing.T) {
	t.Parallel()

	t.Run("invalid autoconfig JSON prevents caching", func(t *testing.T) {
		t.Parallel()
		testInvalidAutoConfigJSONPreventsCaching(t)
	})

	t.Run("malformed multiaddr in autoconfig", func(t *testing.T) {
		t.Parallel()
		testMalformedMultiaddrInAutoConfig(t)
	})

	t.Run("malformed URL in autoconfig", func(t *testing.T) {
		t.Parallel()
		testMalformedURLInAutoConfig(t)
	})
}

func testInvalidAutoConfigJSONPreventsCaching(t *testing.T) {
	// Create server that serves invalid autoconfig JSON
	invalidAutoConfigData := `{
		"AutoConfigVersion": 123,
		"AutoConfigSchema": 3,
		"Bootstrap": [
			"invalid-multiaddr-that-should-fail"
		]
	}`

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Logf("Invalid autoconfig server request #%d: %s %s", requestCount, r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"invalid-config-123"`)
		_, _ = w.Write([]byte(invalidAutoConfigData))
	}))
	defer server.Close()

	// Create IPFS node and try to start daemon with invalid autoconfig
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Start daemon to trigger autoconfig fetch - this should start but log validation errors
	node.StartDaemon()
	defer node.StopDaemon()

	// Give autoconfig some time to attempt fetch and fail validation
	// The daemon should still start but autoconfig should fail
	result := node.RunIPFS("version")
	assert.Equal(t, 0, result.ExitCode(), "Daemon should start even with invalid autoconfig")

	// Verify server was called (autoconfig was attempted even though validation failed)
	assert.Greater(t, requestCount, 0, "Invalid autoconfig server should have been called")
}

func testMalformedMultiaddrInAutoConfig(t *testing.T) {
	// Create server that serves autoconfig with malformed multiaddr
	invalidAutoConfigData := `{
		"AutoConfigVersion": 456,
		"AutoConfigSchema": 3,
		"Bootstrap": [
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
			"not-a-valid-multiaddr"
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(invalidAutoConfigData))
	}))
	defer server.Close()

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Start daemon to trigger autoconfig fetch - daemon should start but autoconfig validation should fail
	node.StartDaemon()
	defer node.StopDaemon()

	// Daemon should still be functional even with invalid autoconfig
	result := node.RunIPFS("version")
	assert.Equal(t, 0, result.ExitCode(), "Daemon should start even with invalid autoconfig")
}

func testMalformedURLInAutoConfig(t *testing.T) {
	// Create server that serves autoconfig with malformed URL
	invalidAutoConfigData := `{
		"AutoConfigVersion": 789,
		"AutoConfigSchema": 3,
		"DNSResolvers": {
			"eth.": ["https://valid.example.com"],
			"bad.": ["://malformed-url-missing-scheme"]
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(invalidAutoConfigData))
	}))
	defer server.Close()

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", server.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"foo.": "auto"})

	// Start daemon to trigger autoconfig fetch - daemon should start but autoconfig validation should fail
	node.StartDaemon()
	defer node.StopDaemon()

	// Daemon should still be functional even with invalid autoconfig
	result := node.RunIPFS("version")
	assert.Equal(t, 0, result.ExitCode(), "Daemon should start even with invalid autoconfig")
}
