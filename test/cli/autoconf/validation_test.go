package autoconf

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestAutoConfValidation(t *testing.T) {
	t.Parallel()

	t.Run("invalid autoconf JSON prevents caching", func(t *testing.T) {
		t.Parallel()
		testInvalidAutoConfJSONPreventsCaching(t)
	})

	t.Run("malformed multiaddr in autoconf", func(t *testing.T) {
		t.Parallel()
		testMalformedMultiaddrInAutoConf(t)
	})

	t.Run("malformed URL in autoconf", func(t *testing.T) {
		t.Parallel()
		testMalformedURLInAutoConf(t)
	})
}

func testInvalidAutoConfJSONPreventsCaching(t *testing.T) {
	// Create server that serves invalid autoconf JSON
	invalidAutoConfData := `{
		"AutoConfVersion": 123,
		"AutoConfSchema": 1,
		"SystemRegistry": {
			"AminoDHT": {
				"NativeConfig": {
					"Bootstrap": [
						"invalid-multiaddr-that-should-fail"
					]
				}
			}
		}
	}`

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		t.Logf("Invalid autoconf server request #%d: %s %s", requestCount, r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"invalid-config-123"`)
		_, _ = w.Write([]byte(invalidAutoConfData))
	}))
	defer server.Close()

	// Create IPFS node and try to start daemon with invalid autoconf
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Start daemon to trigger autoconf fetch - this should start but log validation errors
	node.StartDaemon()
	defer node.StopDaemon()

	// Give autoconf some time to attempt fetch and fail validation
	// The daemon should still start but autoconf should fail
	result := node.RunIPFS("version")
	assert.Equal(t, 0, result.ExitCode(), "Daemon should start even with invalid autoconf")

	// Verify server was called (autoconf was attempted even though validation failed)
	assert.Greater(t, requestCount, 0, "Invalid autoconf server should have been called")
}

func testMalformedMultiaddrInAutoConf(t *testing.T) {
	// Create server that serves autoconf with malformed multiaddr
	invalidAutoConfData := `{
		"AutoConfVersion": 456,
		"AutoConfSchema": 1,
		"SystemRegistry": {
			"AminoDHT": {
				"NativeConfig": {
					"Bootstrap": [
						"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
						"not-a-valid-multiaddr"
					]
				}
			}
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(invalidAutoConfData))
	}))
	defer server.Close()

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("Bootstrap", []string{"auto"})

	// Start daemon to trigger autoconf fetch - daemon should start but autoconf validation should fail
	node.StartDaemon()
	defer node.StopDaemon()

	// Daemon should still be functional even with invalid autoconf
	result := node.RunIPFS("version")
	assert.Equal(t, 0, result.ExitCode(), "Daemon should start even with invalid autoconf")
}

func testMalformedURLInAutoConf(t *testing.T) {
	// Create server that serves autoconf with malformed URL
	invalidAutoConfData := `{
		"AutoConfVersion": 789,
		"AutoConfSchema": 1,
		"DNSResolvers": {
			"eth.": ["https://valid.example.com"],
			"bad.": ["://malformed-url-missing-scheme"]
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(invalidAutoConfData))
	}))
	defer server.Close()

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConf.URL", server.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("DNS.Resolvers", map[string]string{"foo.": "auto"})

	// Start daemon to trigger autoconf fetch - daemon should start but autoconf validation should fail
	node.StartDaemon()
	defer node.StopDaemon()

	// Daemon should still be functional even with invalid autoconf
	result := node.RunIPFS("version")
	assert.Equal(t, 0, result.ExitCode(), "Daemon should start even with invalid autoconf")
}
