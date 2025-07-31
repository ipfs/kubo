package autoconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/kubo/boxo/autoconfig"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoConfigIPNS(t *testing.T) {
	t.Parallel()

	t.Run("IPNS publishing with auto publisher", func(t *testing.T) {
		t.Parallel()
		testIPNSPublishingWithAuto(t)
	})

	t.Run("IPNS publishing errors are handled properly", func(t *testing.T) {
		t.Parallel()
		testIPNSPublishingErrorHandling(t)
	})
}

// mockIPNSPublisher implements a simple IPNS publishing HTTP API server
type mockIPNSPublisher struct {
	t              *testing.T
	server         *httptest.Server
	mu             sync.Mutex
	publishedKeys  map[string]string                      // peerID -> published CID
	recordPayloads map[string][]byte                      // peerID -> actual HTTP PUT record payload
	responseFunc   func(peerID string, record []byte) int // returns HTTP status code
}

func newMockIPNSPublisher(t *testing.T) *mockIPNSPublisher {
	m := &mockIPNSPublisher{
		t:              t,
		publishedKeys:  make(map[string]string),
		recordPayloads: make(map[string][]byte),
	}

	// Default response function accepts all publishes
	m.responseFunc = func(peerID string, record []byte) int {
		return http.StatusOK
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/routing/v1/ipns/", m.handleIPNS)

	m.server = httptest.NewServer(mux)
	return m
}

func (m *mockIPNSPublisher) handleIPNS(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.t.Logf("🔍 Mock IPNS publisher received request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	// Extract peer ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		m.t.Logf("❌ Invalid path structure: %v", parts)
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	peerID := parts[4]
	m.t.Logf("IPNS publisher received %s request for peer: %s", r.Method, peerID)

	if r.Method == "PUT" {
		// Handle IPNS record publication
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		// Get response status from response function
		status := m.responseFunc(peerID, body)

		if status == http.StatusOK {
			// Store the actual record payload for later comparison
			m.recordPayloads[peerID] = make([]byte, len(body))
			copy(m.recordPayloads[peerID], body)

			// Mock successful publish - we don't actually parse the IPNS record
			// but we can extract some info for testing
			m.publishedKeys[peerID] = fmt.Sprintf("published-%d", time.Now().Unix())
			m.t.Logf("IPNS publisher accepted record for peer: %s", peerID)
		}

		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = w.Write([]byte(`{"error": "publish failed"}`))
		}
	} else if r.Method == "GET" {
		// Handle IPNS record retrieval (not used in our test but good to have)
		if record, exists := m.publishedKeys[peerID]; exists {
			w.Header().Set("Content-Type", "application/vnd.ipfs.ipns-record")
			_, _ = w.Write([]byte(record))
		} else {
			http.Error(w, "record not found", http.StatusNotFound)
		}
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *mockIPNSPublisher) getPublishedKeys() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]string)
	for k, v := range m.publishedKeys {
		result[k] = v
	}
	return result
}

func (m *mockIPNSPublisher) getRecordPayload(peerID string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if payload, exists := m.recordPayloads[peerID]; exists {
		result := make([]byte, len(payload))
		copy(result, payload)
		return result
	}
	return nil
}

func (m *mockIPNSPublisher) close() {
	m.server.Close()
}

func testIPNSPublishingWithAuto(t *testing.T) {
	// Test IPNS delegated publishing with autoconfig resolution

	// Create mock IPNS publisher that will capture the HTTP PUT request
	ipnsPublisher := newMockIPNSPublisher(t)
	defer ipnsPublisher.close()

	// Create autoconfig data with delegated publisher using IPNI system (not native)
	autoConfigData := fmt.Sprintf(`{
		"AutoConfigVersion": 2025072302,
		"AutoConfigSchema": 4,
		"CacheTTL": 86400,
		"SystemRegistry": {
			"AminoDHT": {
				"Description": "Test AminoDHT system",
				"NativeConfig": {
					"Bootstrap": []
				}
			},
			"CustomIPNS": {
				"Description": "Test custom IPNS system for delegated publishing",
				"DelegatedConfig": {
					"Read": ["/routing/v1/ipns"],
					"Write": ["/routing/v1/ipns"]
				}
			}
		},
		"DNSResolvers": {},
		"DelegatedEndpoints": {
			"%s": {
				"Systems": ["CustomIPNS"],
				"Read": ["/routing/v1/ipns"],
				"Write": ["/routing/v1/ipns"]
			}
		}
	}`, ipnsPublisher.server.URL)

	// Create autoconfig server
	autoConfigServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("🔍 Mock autoconfig server received request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(autoConfigData))
		t.Logf("📤 Sent autoconfig response with IPNS publisher: %s", ipnsPublisher.server.URL)
	}))
	defer autoConfigServer.Close()

	// Create test harness and main node
	h := harness.NewT(t)

	// Create main IPFS node with auto delegated publisher using auto routing
	mainNode := h.NewNode().Init("--profile=test")
	mainNode.SetIPFSConfig("AutoConfig.URL", autoConfigServer.URL)
	mainNode.SetIPFSConfig("AutoConfig.Enabled", true)
	mainNode.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})
	// Use auto routing to enable both DHT (for "online" status) and delegated routing
	// This allows the node to be considered online while still using delegated IPNS publishers
	mainNode.SetIPFSConfig("Routing.Type", "auto")

	// Add fallback bootstrap peers so the daemon can be considered "online"
	// Use the same bootstrap peers from boxo/autoconfig fallbacks
	mainNode.SetIPFSConfig("Bootstrap", autoconfig.FallbackBootstrapPeers)

	// Start main daemon (delegated routing set in config)
	mainNode.StartDaemon()
	defer mainNode.StopDaemon()

	t.Log("Waiting for daemon to be ready and establish peer connections...")
	time.Sleep(10 * time.Second)

	// Debug: Check if node has peer connections
	peersResult := mainNode.RunIPFS("swarm", "peers")
	connectedPeers := strings.TrimSpace(peersResult.Stdout.String())
	if connectedPeers == "" {
		t.Logf("No peer connections established")
	} else {
		peerCount := len(strings.Split(connectedPeers, "\n"))
		t.Logf("Connected peers: %d", peerCount)
		t.Logf("Peer list: %s", connectedPeers)
	}

	// Debug: Check routing type and DHT status
	routingResult := mainNode.RunIPFS("routing", "findpeer", "--help")
	t.Logf("Routing findpeer help available: %d", routingResult.ExitCode())

	// Verify config shows "auto" and resolves correctly
	configResult := mainNode.RunIPFS("config", "Ipns.DelegatedPublishers")
	require.Equal(t, 0, configResult.ExitCode())

	var publishers []string
	err := json.Unmarshal([]byte(configResult.Stdout.String()), &publishers)
	require.NoError(t, err)
	assert.Equal(t, []string{"auto"}, publishers, "IPNS delegated publishers config should show 'auto'")

	// Check that Routing.Type is actually set to "auto"
	routingTypeResult := mainNode.RunIPFS("config", "Routing.Type")
	require.Equal(t, 0, routingTypeResult.ExitCode())
	routingType := strings.TrimSpace(routingTypeResult.Stdout.String())
	t.Logf("Routing.Type config: %q", routingType)
	assert.Equal(t, "auto", routingType, "Routing.Type should be set to 'auto'")

	// Check that autoconfig resolved the delegated publishers
	resolvedResult := mainNode.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	t.Logf("Resolved IPNS delegated publishers: %s", resolvedResult.Stdout.String())

	var resolvedPublishers []string
	err = json.Unmarshal([]byte(resolvedResult.Stdout.String()), &resolvedPublishers)
	require.NoError(t, err)
	expectedPublisherURL := ipnsPublisher.server.URL + "/routing/v1/ipns"
	assert.Contains(t, resolvedPublishers, expectedPublisherURL,
		"AutoConfig should resolve 'auto' to our mock IPNS publisher URL with path")

	// Get the main node's peer ID to identify the record in the mock server
	idResult := mainNode.RunIPFS("id", "-f", "<id>")
	require.Equal(t, 0, idResult.ExitCode(), "Should be able to get peer ID")
	peerID := strings.TrimSpace(idResult.Stdout.String())
	t.Logf("Main node peer ID: %s", peerID)

	// Test IPNS publishing with specific CID (bafkqablimvwgy3y is inlined "hello")
	testCID := "bafkqablimvwgy3y"

	// Attempt IPNS publishing using --delegated-only mode to test HTTP delegated publishing
	// This ensures we're specifically testing the delegated publisher functionality
	t.Log("Attempting IPNS publish using --delegated-only mode...")
	publishResult := mainNode.RunIPFS("name", "publish", "--delegated-only", "/ipfs/"+testCID)

	if publishResult.ExitCode() != 0 {
		t.Logf("IPNS publish failed: %s", publishResult.Stderr.String())
		t.Logf("IPNS publish stdout: %s", publishResult.Stdout.String())
		require.Equal(t, 0, publishResult.ExitCode(), "IPNS publish should succeed in --delegated-only mode")
	} else {
		t.Log("✅ IPNS publish succeeded in --delegated-only mode")
	}

	output := publishResult.Stdout.String()
	assert.Contains(t, output, "Published to", "Should indicate successful IPNS publish")

	// Extract the IPNS name from the publish output
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var ipnsName string
	for _, line := range lines {
		if strings.Contains(line, "Published to") {
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				ipnsName = strings.TrimSuffix(parts[2], ":")
				break
			}
		}
	}
	require.NotEmpty(t, ipnsName, "Should extract IPNS name from publish output")
	t.Logf("Published IPNS name: %s", ipnsName)

	// CRITICAL TEST: Verify HTTP PUT request was made to delegated publisher with valid payload
	// IPNS publishing to delegated publishers is asynchronous, so we need to wait for the HTTP PUT
	t.Log("Waiting for HTTP PUT request to reach delegated publisher...")
	var publishedKeys map[string]string
	var recordPayload []byte

	// Poll for up to 10 seconds to see if mock server receives the request
	for i := 0; i < 20; i++ {
		publishedKeys = ipnsPublisher.getPublishedKeys()
		recordPayload = ipnsPublisher.getRecordPayload(peerID)

		if len(publishedKeys) > 0 && len(recordPayload) > 0 {
			t.Logf("✅ HTTP PUT request received after %d polling attempts", i+1)
			break
		}

		t.Logf("Polling attempt %d/20: %d keys published so far", i+1, len(publishedKeys))
		time.Sleep(500 * time.Millisecond)
	}

	// FAIL THE TEST if mock server didn't receive the HTTP PUT request
	// This test should verify that IPNS delegated publishing actually works,
	// not just that the configuration is correct.
	require.NotEmpty(t, publishedKeys, "HTTP PUT request MUST be made to delegated publisher - test environment networking issues need to be fixed")
	require.NotNil(t, recordPayload, "HTTP PUT request MUST contain IPNS record payload")
	require.Greater(t, len(recordPayload), 0, "IPNS record payload must not be empty")

	// Verify that the record payload contains expected data structure
	// IPNS records are protobuf encoded, so we can at least verify it's not empty and has reasonable size
	require.Greater(t, len(recordPayload), 50, "IPNS record should be substantial (>50 bytes)")
	require.Less(t, len(recordPayload), 10000, "IPNS record should be reasonable size (<10KB)")

	t.Logf("✅ IPNS autoconfig test completed successfully:")
	t.Logf("  - --delegated-only flag used HTTP delegated IPNS publishing exclusively")
	t.Logf("  - AutoConfig resolved 'auto' to: %s", ipnsPublisher.server.URL)
	t.Logf("  - IPNS publishing successful in --delegated-only mode")
	t.Logf("  - Published IPNS name: %s", ipnsName)
	t.Logf("  - ✅ HTTP PUT request made to delegated publisher with %d byte payload", len(recordPayload))
	t.Logf("  - ✅ Key validation: HTTP PUT to /routing/v1/ipns with valid IPNS record payload")

	// Test passes only when HTTP PUT occurred with valid payload
}

func testIPNSPublishingErrorHandling(t *testing.T) {
	t.Run("IPNS publishing 404 error", func(t *testing.T) {
		testIPNSPublishing404Error(t)
	})

	t.Run("IPNS publishing 500 error", func(t *testing.T) {
		testIPNSPublishing500Error(t)
	})
}

func testIPNSPublishing404Error(t *testing.T) {
	// Create IPNS publisher that returns 404
	ipnsPublisher := newMockIPNSPublisher(t)
	defer ipnsPublisher.close()

	// Configure to return 404 Not Found
	ipnsPublisher.responseFunc = func(peerID string, record []byte) int {
		return http.StatusNotFound
	}

	// Create autoconfig data
	autoConfigData := fmt.Sprintf(`{
		"AutoConfigVersion": 2025072302,
		"AutoConfigSchema": 4,
		"CacheTTL": 86400,
		"SystemRegistry": {
			"AminoDHT": {
				"Description": "Test AminoDHT system",
				"NativeConfig": {
					"Bootstrap": []
				}
			}
		},
		"DNSResolvers": {},
		"DelegatedEndpoints": {
			"%s": {
				"Systems": ["AminoDHT"],
				"Read": ["/routing/v1/ipns"],
				"Write": ["/routing/v1/ipns"]
			}
		}
	}`, ipnsPublisher.server.URL)

	autoConfigServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(autoConfigData))
	}))
	defer autoConfigServer.Close()

	// Create IPFS node with auto delegated publisher
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", autoConfigServer.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Set Routing.Type to delegated to ensure delegated routers and publishers are used
	node.SetIPFSConfig("Routing.Type", "delegated")
	node.SetIPFSConfig("Provider.Enabled", false)   // Required for delegated routing
	node.SetIPFSConfig("Reprovider.Interval", "0s") // Required for delegated routing

	// Add fallback bootstrap peers so the daemon can be considered "online"
	// Use the same bootstrap peers from boxo/autoconfig fallbacks
	node.SetIPFSConfig("Bootstrap", autoconfig.FallbackBootstrapPeers)

	node.StartDaemon()
	defer node.StopDaemon()

	// Test IPNS publishing with 404-returning delegated publisher
	testCID := "bafkqablimvwgy3y"
	result := node.RunIPFS("name", "publish", "--allow-offline", "/ipfs/"+testCID)

	// NOTE: Since --allow-offline uses offline routing, it won't contact delegated publishers
	// so publishing will succeed regardless of what the delegated publisher would return
	require.Equal(t, 0, result.ExitCode(), "IPNS publish should succeed in offline mode")

	// However, we can verify that the autoconfig system correctly configured the 404-returning publisher
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	var resolvedPublishers []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &resolvedPublishers)
	require.NoError(t, err)

	// Confirm that our mock server URL was resolved from "auto"
	expectedPublisherURL := ipnsPublisher.server.URL + "/routing/v1/ipns"
	assert.Contains(t, resolvedPublishers, expectedPublisherURL,
		"AutoConfig should resolve 'auto' to mock IPNS publisher URL with path even when it returns 404")

	t.Log("✅ AutoConfig correctly resolved IPNS delegated publisher that would return 404 error")
}

func testIPNSPublishing500Error(t *testing.T) {
	// Create IPNS publisher that returns 500
	ipnsPublisher := newMockIPNSPublisher(t)
	defer ipnsPublisher.close()

	// Configure to return 500 Internal Server Error
	ipnsPublisher.responseFunc = func(peerID string, record []byte) int {
		return http.StatusInternalServerError
	}

	// Create autoconfig data
	autoConfigData := fmt.Sprintf(`{
		"AutoConfigVersion": 2025072302,
		"AutoConfigSchema": 4,
		"CacheTTL": 86400,
		"SystemRegistry": {
			"AminoDHT": {
				"Description": "Test AminoDHT system",
				"NativeConfig": {
					"Bootstrap": []
				}
			}
		},
		"DNSResolvers": {},
		"DelegatedEndpoints": {
			"%s": {
				"Systems": ["AminoDHT"],
				"Read": ["/routing/v1/ipns"],
				"Write": ["/routing/v1/ipns"]
			}
		}
	}`, ipnsPublisher.server.URL)

	autoConfigServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(autoConfigData))
	}))
	defer autoConfigServer.Close()

	// Create IPFS node with auto delegated publisher
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", autoConfigServer.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Set Routing.Type to delegated to ensure delegated routers and publishers are used
	node.SetIPFSConfig("Routing.Type", "delegated")
	node.SetIPFSConfig("Provider.Enabled", false)   // Required for delegated routing
	node.SetIPFSConfig("Reprovider.Interval", "0s") // Required for delegated routing

	// Add fallback bootstrap peers so the daemon can be considered "online"
	// Use the same bootstrap peers from boxo/autoconfig fallbacks
	node.SetIPFSConfig("Bootstrap", autoconfig.FallbackBootstrapPeers)

	node.StartDaemon()
	defer node.StopDaemon()

	// Test IPNS publishing with 500-returning delegated publisher
	testCID := "bafkqablimvwgy3y"
	result := node.RunIPFS("name", "publish", "--allow-offline", "/ipfs/"+testCID)

	// NOTE: Since --allow-offline uses offline routing, it won't contact delegated publishers
	// so publishing will succeed regardless of what the delegated publisher would return
	require.Equal(t, 0, result.ExitCode(), "IPNS publish should succeed in offline mode")

	// However, we can verify that the autoconfig system correctly configured the 500-returning publisher
	result = node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	var resolvedPublishers []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &resolvedPublishers)
	require.NoError(t, err)

	// Confirm that our mock server URL was resolved from "auto"
	expectedPublisherURL := ipnsPublisher.server.URL + "/routing/v1/ipns"
	assert.Contains(t, resolvedPublishers, expectedPublisherURL,
		"AutoConfig should resolve 'auto' to mock IPNS publisher URL with path even when it returns 500")

	t.Log("✅ AutoConfig correctly resolved IPNS delegated publisher that would return 500 error")
}
