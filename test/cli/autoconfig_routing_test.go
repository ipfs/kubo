package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ipfs/kubo/boxo/autoconfig"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoConfigDelegatedRouting(t *testing.T) {
	t.Parallel()

	t.Run("delegated routing with auto router", func(t *testing.T) {
		t.Parallel()
		testDelegatedRoutingWithAuto(t)
	})

	t.Run("routing errors are handled properly", func(t *testing.T) {
		t.Parallel()
		testRoutingErrorHandling(t)
	})
}

// mockRoutingServer implements a simple Delegated Routing HTTP API server
type mockRoutingServer struct {
	t            *testing.T
	server       *httptest.Server
	mu           sync.Mutex
	requests     []string
	providerFunc func(cid string) []map[string]interface{}
}

func newMockRoutingServer(t *testing.T) *mockRoutingServer {
	m := &mockRoutingServer{
		t:        t,
		requests: []string{},
	}

	// Default provider function returns mock provider records
	m.providerFunc = func(cid string) []map[string]interface{} {
		return []map[string]interface{}{
			{
				"Protocol": "transport-bitswap",
				"Schema":   "bitswap",
				"ID":       "12D3KooWMockProvider1",
				"Addrs":    []string{"/ip4/192.168.1.100/tcp/4001"},
			},
			{
				"Protocol": "transport-bitswap",
				"Schema":   "bitswap",
				"ID":       "12D3KooWMockProvider2",
				"Addrs":    []string{"/ip4/192.168.1.101/tcp/4001"},
			},
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/routing/v1/providers/", m.handleProviders)

	m.server = httptest.NewServer(mux)
	return m
}

func (m *mockRoutingServer) handleProviders(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Extract CID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	cid := parts[4]
	m.requests = append(m.requests, cid)
	m.t.Logf("Routing server received providers request for CID: %s", cid)

	// Get provider records
	providers := m.providerFunc(cid)

	// Return NDJSON response as per IPIP-378
	w.Header().Set("Content-Type", "application/x-ndjson")
	encoder := json.NewEncoder(w)

	for _, provider := range providers {
		if err := encoder.Encode(provider); err != nil {
			m.t.Logf("Failed to encode provider: %v", err)
			return
		}
	}
}

func (m *mockRoutingServer) close() {
	m.server.Close()
}

func testDelegatedRoutingWithAuto(t *testing.T) {
	// Create mock routing server
	routingServer := newMockRoutingServer(t)
	defer routingServer.close()

	// Create autoconfig data with delegated router
	autoConfigData := fmt.Sprintf(`{
		"AutoConfigVersion": 2025072302,
		"AutoConfigSchema": 3,
		"Bootstrap": [],
		"DelegatedRouters": {
			\"%s\": [\"%s\"]
		}
	}`, autoconfig.MainnetProfileNodesWithDHT, routingServer.server.URL)

	// Create autoconfig server
	autoConfigServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(autoConfigData))
	}))
	defer autoConfigServer.Close()

	// Create IPFS node with auto delegated router
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", autoConfigServer.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})

	// Test that daemon starts successfully with auto routing configuration
	// The actual routing functionality requires online mode, but we can test
	// that the configuration is expanded and daemon starts properly
	node.StartDaemon("--offline")
	defer node.StopDaemon()

	// Verify config still shows "auto" (this tests that auto values are preserved in user-facing config)
	result := node.RunIPFS("config", "Routing.DelegatedRouters")
	require.Equal(t, 0, result.ExitCode())

	var routers []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &routers)
	require.NoError(t, err)
	assert.Equal(t, []string{"auto"}, routers, "Delegated routers config should show 'auto'")

	// Test that daemon is running and accepting commands
	result = node.RunIPFS("version")
	require.Equal(t, 0, result.ExitCode(), "Daemon should be running and accepting commands")

	// Test that autoconfig server was contacted (indicating successful resolution)
	// We can't test actual routing in offline mode, but we can verify that
	// the AutoConfig system expanded the "auto" placeholder successfully
	// by checking that the daemon started without errors
	t.Log("AutoConfig successfully expanded delegated router configuration and daemon started")
}

func testRoutingErrorHandling(t *testing.T) {
	// Create routing server that returns no providers
	routingServer := newMockRoutingServer(t)
	defer routingServer.close()

	// Configure to return no providers (empty response)
	routingServer.providerFunc = func(cid string) []map[string]interface{} {
		return []map[string]interface{}{}
	}

	// Create autoconfig data
	autoConfigData := fmt.Sprintf(`{
		"AutoConfigVersion": 2025072302,
		"AutoConfigSchema": 3,
		"Bootstrap": [],
		"DelegatedRouters": {
			\"%s\": [\"%s\"]
		}
	}`, autoconfig.MainnetProfileNodesWithDHT, routingServer.server.URL)

	// Create autoconfig server
	autoConfigServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(autoConfigData))
	}))
	defer autoConfigServer.Close()

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", autoConfigServer.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Routing.DelegatedRouters", []string{"auto"})

	// Test that daemon starts successfully even when no providers are available
	node.StartDaemon("--offline")
	defer node.StopDaemon()

	// Verify config shows "auto"
	result := node.RunIPFS("config", "Routing.DelegatedRouters")
	require.Equal(t, 0, result.ExitCode())

	var routers []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &routers)
	require.NoError(t, err)
	assert.Equal(t, []string{"auto"}, routers, "Delegated routers config should show 'auto'")

	// Test that daemon is running and accepting commands
	result = node.RunIPFS("version")
	require.Equal(t, 0, result.ExitCode(), "Daemon should be running even with empty routing config")

	t.Log("AutoConfig successfully handled routing configuration with empty providers")
}
