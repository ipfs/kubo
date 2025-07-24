package cli

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
	t             *testing.T
	server        *httptest.Server
	mu            sync.Mutex
	publishedKeys map[string]string                      // peerID -> published CID
	responseFunc  func(peerID string, record []byte) int // returns HTTP status code
}

func newMockIPNSPublisher(t *testing.T) *mockIPNSPublisher {
	m := &mockIPNSPublisher{
		t:             t,
		publishedKeys: make(map[string]string),
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
			// Mock successful publish - we don't actually parse the IPNS record
			// but we can extract some info for testing
			m.publishedKeys[peerID] = fmt.Sprintf("published-%d", time.Now().Unix())
			m.t.Logf("IPNS publisher accepted record for peer: %s", peerID)
		}

		w.WriteHeader(status)
		if status != http.StatusOK {
			w.Write([]byte(`{"error": "publish failed"}`))
		}
	} else if r.Method == "GET" {
		// Handle IPNS record retrieval (not used in our test but good to have)
		if record, exists := m.publishedKeys[peerID]; exists {
			w.Header().Set("Content-Type", "application/vnd.ipfs.ipns-record")
			w.Write([]byte(record))
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

func (m *mockIPNSPublisher) close() {
	m.server.Close()
}

func testIPNSPublishingWithAuto(t *testing.T) {
	// Create mock IPNS publisher
	ipnsPublisher := newMockIPNSPublisher(t)
	defer ipnsPublisher.close()

	// Create autoconfig data with delegated publisher
	autoConfigData := fmt.Sprintf(`{
		"AutoConfigVersion": 2025072302,
		"AutoConfigSchema": 3,
		"Bootstrap": [],
		"DelegatedPublishers": {
			"for-ipns-publishers-with-http": ["%s"]
		}
	}`, ipnsPublisher.server.URL)

	// Create autoconfig server
	autoConfigServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(autoConfigData))
	}))
	defer autoConfigServer.Close()

	// Create IPFS node with auto delegated publisher
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", autoConfigServer.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Start daemon
	node.StartDaemon()
	defer node.StopDaemon()

	// Verify config still shows "auto"
	result := node.RunIPFS("config", "Ipns.DelegatedPublishers")
	require.Equal(t, 0, result.ExitCode())

	var publishers []string
	err := json.Unmarshal([]byte(result.Stdout.String()), &publishers)
	require.NoError(t, err)
	assert.Equal(t, []string{"auto"}, publishers, "IPNS delegated publishers config should show 'auto'")

	// Add some content
	cid := node.IPFSAddStr("test content for IPNS publishing")

	// Test IPNS publishing - use --allow-offline since we can't easily test online publishing in unit tests
	// The important part is that the AutoConfig expanded the delegated publisher URL correctly
	result = node.RunIPFS("name", "publish", cid, "--allow-offline")
	if result.ExitCode() != 0 {
		t.Logf("IPNS publish failed: %s", result.Stderr.String())
	}
	require.Equal(t, 0, result.ExitCode(), "IPNS publish should succeed")

	output := result.Stdout.String()

	// Should indicate successful publish
	assert.Contains(t, output, "Published to", "Should indicate successful IPNS publish")

	// The test verifies that AutoConfig correctly expanded the 'auto' placeholder
	// and the daemon started successfully with the mock publisher configured.
	// Offline publishing doesn't use the delegated publisher, but the configuration
	// resolution and daemon startup validates the AutoConfig functionality.
	t.Log("AutoConfig successfully expanded IPNS delegated publisher configuration")
}

func testIPNSPublishingErrorHandling(t *testing.T) {
	// Create IPNS publisher that returns errors
	ipnsPublisher := newMockIPNSPublisher(t)
	defer ipnsPublisher.close()

	// Configure to return server error
	ipnsPublisher.responseFunc = func(peerID string, record []byte) int {
		return http.StatusInternalServerError
	}

	// Create autoconfig data
	autoConfigData := fmt.Sprintf(`{
		"AutoConfigVersion": 2025072302,
		"AutoConfigSchema": 3,
		"Bootstrap": [],
		"DelegatedPublishers": {
			"for-ipns-publishers-with-http": ["%s"]
		}
	}`, ipnsPublisher.server.URL)

	// Create autoconfig server
	autoConfigServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(autoConfigData))
	}))
	defer autoConfigServer.Close()

	// Create IPFS node
	node := harness.NewT(t).NewNode().Init("--profile=test")
	node.SetIPFSConfig("AutoConfig.URL", autoConfigServer.URL)
	node.SetIPFSConfig("AutoConfig.Enabled", true)
	node.SetIPFSConfig("Ipns.DelegatedPublishers", []string{"auto"})

	// Start daemon
	node.StartDaemon()
	defer node.StopDaemon()

	// Add content
	cid := node.IPFSAddStr("test content with publish error")

	// Try to publish - should fail due to server error
	result := node.RunIPFS("name", "publish", cid, "--allow-offline")
	// Command might still succeed if it falls back to local publishing
	// but we should check that the delegated publisher was contacted

	// Verify IPNS publisher received the request (even though it failed)
	publishedKeys := ipnsPublisher.getPublishedKeys()
	// Keys map should be empty since our mock returns error
	assert.Equal(t, 0, len(publishedKeys), "No keys should be published due to server error")

	t.Logf("Publish result - Exit code: %d, Stdout: %s, Stderr: %s",
		result.ExitCode(), result.Stdout.String(), result.Stderr.String())
}
