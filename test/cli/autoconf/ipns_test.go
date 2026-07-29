package autoconf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/autoconf"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAutoConfIPNS tests IPNS publishing with autoconf-resolved delegated publishers
func TestAutoConfIPNS(t *testing.T) {
	t.Parallel()

	t.Run("PublishingWithWorkingEndpoint", func(t *testing.T) {
		t.Parallel()
		testIPNSPublishingWithWorkingEndpoint(t)
	})

	t.Run("PublishingResilience", func(t *testing.T) {
		t.Parallel()
		testIPNSPublishingResilience(t)
	})
}

// testIPNSPublishingWithWorkingEndpoint verifies that IPNS delegated publishing works
// correctly when the HTTP endpoint is functioning normally and accepts requests.
// It also verifies that the PUT payload matches what can be retrieved via routing get.
func testIPNSPublishingWithWorkingEndpoint(t *testing.T) {
	// Create mock IPNS publisher that accepts requests
	publisher := newMockIPNSPublisher(t)
	defer publisher.close()

	// Create node with delegated publisher
	node := setupNodeWithAutoconf(t, publisher.server.URL, "auto")
	defer node.StopDaemon()

	// Wait for daemon to be ready
	time.Sleep(5 * time.Second)

	// Get node's peer ID
	idResult := node.RunIPFS("id", "-f", "<id>")
	require.Equal(t, 0, idResult.ExitCode())
	peerID := strings.TrimSpace(idResult.Stdout.String())

	// Get peer ID in base36 format (used for IPNS keys)
	idBase36Result := node.RunIPFS("id", "--peerid-base", "base36", "-f", "<id>")
	require.Equal(t, 0, idBase36Result.ExitCode())
	peerIDBase36 := strings.TrimSpace(idBase36Result.Stdout.String())

	// Verify autoconf resolved "auto" correctly
	result := node.RunIPFS("config", "Ipns.DelegatedPublishers", "--expand-auto")
	var resolvedPublishers []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &resolvedPublishers)
	require.NoError(t, err)
	expectedURL := publisher.server.URL + "/routing/v1/ipns"
	assert.Contains(t, resolvedPublishers, expectedURL, "AutoConf should resolve 'auto' to mock publisher")

	// Test publishing with --allow-delegated
	testCID := "bafkqablimvwgy3y"
	result = node.RunIPFS("name", "publish", "--allow-delegated", "/ipfs/"+testCID)
	require.Equal(t, 0, result.ExitCode(), "Publishing should succeed")
	assert.Contains(t, result.Stdout.String(), "Published to")

	// Wait for the async HTTP request to reach the delegated publisher
	require.Eventually(t, func() bool {
		return len(publisher.getRecordPayloads(peerIDBase36)) > 0
	}, 30*time.Second, 100*time.Millisecond, "HTTP PUT request should have been made to delegated publisher")

	// Verify HTTP PUT was made to delegated publisher
	publishedKeys := publisher.getPublishedKeys()
	assert.NotEmpty(t, publishedKeys, "HTTP PUT request should have been made to delegated publisher")

	// Retrieve the IPNS record using routing get
	getResult := node.RunIPFS("routing", "get", "/ipns/"+peerID)
	require.Equal(t, 0, getResult.ExitCode(), "Should be able to retrieve IPNS record")
	getPayload := getResult.Stdout.Bytes()

	// The record routing returns has to be one we PUT to the delegated
	// publisher, not necessarily the first. A minute after startup the
	// republisher re-signs every key and publishes it again, which yields a
	// different signature and expiry for the same value, and routing prefers
	// that newer record. Waiting lets the matching PUT arrive if it has not yet.
	require.Eventually(t, func() bool {
		return slices.ContainsFunc(publisher.getRecordPayloads(peerIDBase36), func(payload []byte) bool {
			return bytes.Equal(payload, getPayload)
		})
	}, 30*time.Second, 200*time.Millisecond,
		"record returned by routing get should be one of the records PUT to the delegated publisher")

	// Also verify the record points to the expected content
	assert.Contains(t, getResult.Stdout.String(), testCID,
		"Retrieved IPNS record should reference the published CID")

	// Use ipfs name inspect to verify the IPNS record's value matches the published CID
	// First write the routing get result to a file for inspection
	node.WriteBytes("ipns-record", getPayload)
	inspectResult := node.RunIPFS("name", "inspect", "ipns-record")
	require.Equal(t, 0, inspectResult.ExitCode(), "Should be able to inspect IPNS record")

	// The inspect output should show the path we published
	inspectOutput := inspectResult.Stdout.String()
	assert.Contains(t, inspectOutput, "/ipfs/"+testCID,
		"IPNS record value should match the published path")

	// Also verify it's a valid record with proper fields
	assert.Contains(t, inspectOutput, "Value:", "Should have Value field")
	assert.Contains(t, inspectOutput, "Validity:", "Should have Validity field")
	assert.Contains(t, inspectOutput, "Sequence:", "Should have Sequence field")

	t.Log("Verified: PUT payload to delegated publisher matches routing get result and name inspect confirms correct path")
}

// testIPNSPublishingResilience verifies that IPNS publishing is resilient by design.
// Publishing succeeds as long as local storage works, even when all delegated endpoints fail.
// This test documents the intentional resilient behavior, not bugs.
func testIPNSPublishingResilience(t *testing.T) {
	testCases := []struct {
		name        string
		routingType string // "auto" or "delegated"
		description string
	}{
		{
			name:        "AutoRouting",
			routingType: "auto",
			description: "auto mode uses DHT + HTTP, tolerates HTTP failures",
		},
		{
			name:        "DelegatedRouting",
			routingType: "delegated",
			description: "delegated mode uses HTTP only, tolerates HTTP failures",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create publisher that always fails
			publisher := newMockIPNSPublisher(t)
			defer publisher.close()
			publisher.responseFunc = func(peerID string, record []byte) int {
				return http.StatusInternalServerError
			}

			// Create node with failing endpoint
			node := setupNodeWithAutoconf(t, publisher.server.URL, tc.routingType)
			defer node.StopDaemon()

			// Test different publishing modes - all should succeed due to resilient design
			testCID := "/ipfs/bafkqablimvwgy3y"

			// Normal publishing (should succeed despite endpoint failures)
			result := node.RunIPFS("name", "publish", testCID)
			assert.Equal(t, 0, result.ExitCode(),
				"%s: Normal publishing should succeed (local storage works)", tc.description)

			// Publishing with --allow-offline (local only, no network)
			result = node.RunIPFS("name", "publish", "--allow-offline", testCID)
			assert.Equal(t, 0, result.ExitCode(),
				"--allow-offline should succeed (local only)")

			// Publishing with --allow-delegated (if using auto routing)
			if tc.routingType == "auto" {
				result = node.RunIPFS("name", "publish", "--allow-delegated", testCID)
				assert.Equal(t, 0, result.ExitCode(),
					"--allow-delegated should succeed (no DHT required)")
			}

			t.Logf("%s: All publishing modes succeeded despite endpoint failures (resilient design)", tc.name)
		})
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// setupNodeWithAutoconf creates an IPFS node with autoconf-configured delegated publishers
func setupNodeWithAutoconf(t *testing.T, publisherURL string, routingType string) *harness.Node {
	// Create autoconf server with the publisher endpoint
	autoconfData := createAutoconfJSON(publisherURL)
	autoconfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, autoconfData)
	}))
	t.Cleanup(func() { autoconfServer.Close() })

	// Create and configure node
	h := harness.NewT(t)
	node := h.NewNode().Init("--profile=test")

	// Configure autoconf
	node.SetIPFSConfig("AutoConf.URL", autoconfServer.URL)
	node.SetIPFSConfig("AutoConf.Enabled", true)
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})
	node.SetIPFSConfig("Routing.Type", routingType)

	// Additional config for delegated routing mode
	if routingType == "delegated" {
		node.SetIPFSConfig("Provide.Enabled", false)
		node.SetIPFSConfig("Provide.DHT.Interval", "0s")
	}

	// Add bootstrap peers for connectivity
	node.SetIPFSConfig("Bootstrap", autoconf.FallbackBootstrapPeers)

	// Start daemon
	node.StartDaemon()

	return node
}

// createAutoconfJSON generates autoconf configuration with a delegated IPNS publisher
func createAutoconfJSON(publisherURL string) string {
	// Use bootstrap peers from autoconf fallbacks for consistency
	bootstrapPeers, _ := json.Marshal(autoconf.FallbackBootstrapPeers)

	return fmt.Sprintf(`{
		"AutoConfVersion": 2025072302,
		"AutoConfSchema": 1,
		"AutoConfTTL": 86400,
		"SystemRegistry": {
			"TestSystem": {
				"Description": "Test system for IPNS publishing",
				"NativeConfig": {
					"Bootstrap": %s
				}
			}
		},
		"DNSResolvers": {},
		"DelegatedEndpoints": {
			"%s": {
				"Systems": ["TestSystem"],
				"Read": ["/routing/v1/ipns"],
				"Write": ["/routing/v1/ipns"]
			}
		}
	}`, string(bootstrapPeers), publisherURL)
}

// ============================================================================
// Mock IPNS Publisher
// ============================================================================

// mockIPNSPublisher implements a simple IPNS publishing HTTP API server
type mockIPNSPublisher struct {
	t              *testing.T
	server         *httptest.Server
	mu             sync.Mutex
	publishedKeys  map[string]string                      // peerID -> published CID
	recordPayloads map[string][][]byte                    // peerID -> every HTTP PUT record payload, in order
	responseFunc   func(peerID string, record []byte) int // returns HTTP status code
}

func newMockIPNSPublisher(t *testing.T) *mockIPNSPublisher {
	m := &mockIPNSPublisher{
		t:              t,
		publishedKeys:  make(map[string]string),
		recordPayloads: make(map[string][][]byte),
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

	// Extract peer ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	peerID := parts[4]

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
			if len(body) > 0 {
				// Keep every record we are sent, not just the last one. The
				// republisher re-signs and re-publishes on its own schedule.
				m.recordPayloads[peerID] = append(m.recordPayloads[peerID], bytes.Clone(body))
			}

			// Mark as published
			m.publishedKeys[peerID] = fmt.Sprintf("published-%d", time.Now().Unix())
		}

		w.WriteHeader(status)
		if status != http.StatusOK {
			fmt.Fprint(w, `{"error": "publish failed"}`)
		}
	} else if r.Method == "GET" {
		// Handle IPNS record retrieval
		if record, exists := m.publishedKeys[peerID]; exists {
			w.Header().Set("Content-Type", "application/vnd.ipfs.ipns-record")
			fmt.Fprint(w, record)
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
	maps.Copy(result, m.publishedKeys)
	return result
}

func (m *mockIPNSPublisher) getRecordPayloads(peerID string) [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	payloads := make([][]byte, 0, len(m.recordPayloads[peerID]))
	for _, payload := range m.recordPayloads[peerID] {
		payloads = append(payloads, bytes.Clone(payload))
	}
	return payloads
}

func (m *mockIPNSPublisher) close() {
	m.server.Close()
}
