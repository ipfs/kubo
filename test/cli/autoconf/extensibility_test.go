package autoconf

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// TestAutoConfExtensibility_NewSystem verifies that the AutoConf system can be extended
// with new routing systems beyond the default AminoDHT and IPNI.
//
// The test verifies that:
// 1. New systems can be added via AutoConf's SystemRegistry
// 2. Native vs delegated system filtering works correctly:
//   - Native systems (AminoDHT) provide bootstrap peers and are used for P2P routing
//   - Delegated systems (IPNI, NewSystem) provide HTTP endpoints for delegated routing
//
// 3. The system correctly filters endpoints based on routing type
//
// Note: Only native systems contribute bootstrap peers. Delegated systems like "NewSystem"
// only provide HTTP routing endpoints, not P2P bootstrap peers.
func TestAutoConfExtensibility_NewSystem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Setup mock autoconf server with NewSystem
	var mockServer *httptest.Server
	mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create autoconf.json with NewSystem
		autoconfData := map[string]interface{}{
			"AutoConfVersion": 2025072901,
			"AutoConfSchema":  1,
			"AutoConfTTL":     86400,
			"SystemRegistry": map[string]interface{}{
				"AminoDHT": map[string]interface{}{
					"URL":         "https://github.com/ipfs/specs/pull/497",
					"Description": "Public DHT swarm",
					"NativeConfig": map[string]interface{}{
						"Bootstrap": []string{
							"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
						},
					},
					"DelegatedConfig": map[string]interface{}{
						"Read":  []string{"/routing/v1/providers", "/routing/v1/peers", "/routing/v1/ipns"},
						"Write": []string{"/routing/v1/ipns"},
					},
				},
				"IPNI": map[string]interface{}{
					"URL":         "https://ipni.example.com",
					"Description": "Network Indexer",
					"DelegatedConfig": map[string]interface{}{
						"Read":  []string{"/routing/v1/providers"},
						"Write": []string{},
					},
				},
				"NewSystem": map[string]interface{}{
					"URL":         "https://example.com/newsystem",
					"Description": "Test system for extensibility verification",
					"NativeConfig": map[string]interface{}{
						"Bootstrap": []string{
							"/ip4/127.0.0.1/tcp/9999/p2p/12D3KooWPeQ4r3v6CmVmKXoFGtqEqcr3L8P6La9yH5oEWKtoLVVa",
						},
					},
					"DelegatedConfig": map[string]interface{}{
						"Read":  []string{"/routing/v1/providers"},
						"Write": []string{},
					},
				},
			},
			"DNSResolvers": map[string]interface{}{
				"eth.": []string{"https://dns.eth.limo/dns-query"},
			},
			"DelegatedEndpoints": map[string]interface{}{
				"https://ipni.example.com": map[string]interface{}{
					"Systems": []string{"IPNI"},
					"Read":    []string{"/routing/v1/providers"},
					"Write":   []string{},
				},
				mockServer.URL + "/newsystem": map[string]interface{}{
					"Systems": []string{"NewSystem"},
					"Read":    []string{"/routing/v1/providers"},
					"Write":   []string{},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=300")
		_ = json.NewEncoder(w).Encode(autoconfData)
	}))
	defer mockServer.Close()

	// NewSystem mock server URL will be dynamically assigned
	newSystemServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple mock server for NewSystem endpoint
		response := map[string]interface{}{"Providers": []interface{}{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer newSystemServer.Close()

	// Update the autoconf to point to the correct NewSystem endpoint
	mockServer.Close()
	mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		autoconfData := map[string]interface{}{
			"AutoConfVersion": 2025072901,
			"AutoConfSchema":  1,
			"AutoConfTTL":     86400,
			"SystemRegistry": map[string]interface{}{
				"AminoDHT": map[string]interface{}{
					"URL":         "https://github.com/ipfs/specs/pull/497",
					"Description": "Public DHT swarm",
					"NativeConfig": map[string]interface{}{
						"Bootstrap": []string{
							"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
						},
					},
					"DelegatedConfig": map[string]interface{}{
						"Read":  []string{"/routing/v1/providers", "/routing/v1/peers", "/routing/v1/ipns"},
						"Write": []string{"/routing/v1/ipns"},
					},
				},
				"IPNI": map[string]interface{}{
					"URL":         "https://ipni.example.com",
					"Description": "Network Indexer",
					"DelegatedConfig": map[string]interface{}{
						"Read":  []string{"/routing/v1/providers"},
						"Write": []string{},
					},
				},
				"NewSystem": map[string]interface{}{
					"URL":         "https://example.com/newsystem",
					"Description": "Test system for extensibility verification",
					"NativeConfig": map[string]interface{}{
						"Bootstrap": []string{
							"/ip4/127.0.0.1/tcp/9999/p2p/12D3KooWPeQ4r3v6CmVmKXoFGtqEqcr3L8P6La9yH5oEWKtoLVVa",
						},
					},
					"DelegatedConfig": map[string]interface{}{
						"Read":  []string{"/routing/v1/providers"},
						"Write": []string{},
					},
				},
			},
			"DNSResolvers": map[string]interface{}{
				"eth.": []string{"https://dns.eth.limo/dns-query"},
			},
			"DelegatedEndpoints": map[string]interface{}{
				"https://ipni.example.com": map[string]interface{}{
					"Systems": []string{"IPNI"},
					"Read":    []string{"/routing/v1/providers"},
					"Write":   []string{},
				},
				newSystemServer.URL: map[string]interface{}{
					"Systems": []string{"NewSystem"},
					"Read":    []string{"/routing/v1/providers"},
					"Write":   []string{},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=300")
		_ = json.NewEncoder(w).Encode(autoconfData)
	}))
	defer mockServer.Close()

	// Create Kubo node with autoconf pointing to mock server
	h := harness.NewT(t)
	node := h.NewNode().Init()

	// Update config to use mock autoconf server
	node.UpdateConfig(func(cfg *config.Config) {
		cfg.AutoConf.URL = config.NewOptionalString(mockServer.URL)
		cfg.AutoConf.Enabled = config.True
		cfg.AutoConf.RefreshInterval = config.NewOptionalDuration(1 * time.Second)
		cfg.Routing.Type = config.NewOptionalString("auto") // Should enable native AminoDHT + delegated others
		cfg.Bootstrap = []string{"auto"}
		cfg.Routing.DelegatedRouters = []string{"auto"}
	})

	// Start the daemon
	daemon := node.StartDaemon()
	defer daemon.StopDaemon()

	// Give the daemon some time to initialize and make requests
	time.Sleep(3 * time.Second)

	// Test 1: Verify bootstrap includes both AminoDHT and NewSystem peers (deduplicated)
	bootstrapResult := daemon.IPFS("bootstrap", "list", "--expand-auto")
	bootstrapOutput := bootstrapResult.Stdout.String()
	t.Logf("Bootstrap output: %s", bootstrapOutput)

	// Should contain original DHT bootstrap peer (AminoDHT is a native system)
	require.Contains(t, bootstrapOutput, "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN", "Should contain AminoDHT bootstrap peer")

	// Note: NewSystem bootstrap peers are NOT included because only native systems
	// (AminoDHT for Routing.Type="auto") contribute bootstrap peers.
	// Delegated systems like NewSystem only provide HTTP routing endpoints.

	// Test 2: Verify delegated endpoints are filtered correctly
	// For Routing.Type=auto, native systems=[AminoDHT], so:
	// - AminoDHT endpoints should be filtered out
	// - IPNI and NewSystem endpoints should be included

	// Get the expanded delegated routers using --expand-auto
	routerResult := daemon.IPFS("config", "Routing.DelegatedRouters", "--expand-auto")
	var expandedRouters []string
	require.NoError(t, json.Unmarshal([]byte(routerResult.Stdout.String()), &expandedRouters))

	t.Logf("Expanded delegated routers: %v", expandedRouters)

	// Verify we got exactly 2 delegated routers: IPNI and NewSystem
	require.Equal(t, 2, len(expandedRouters), "Should have exactly 2 delegated routers (IPNI and NewSystem). Got %d: %v", len(expandedRouters), expandedRouters)

	// Convert to URLs for checking
	routerURLs := expandedRouters

	// Should contain NewSystem endpoint (not native) - now with routing path
	foundNewSystem := false
	expectedNewSystemURL := newSystemServer.URL + "/routing/v1/providers" // Full URL with path, as returned by DelegatedRoutersWithAutoConf
	for _, url := range routerURLs {
		if url == expectedNewSystemURL {
			foundNewSystem = true
			break
		}
	}
	require.True(t, foundNewSystem, "Should contain NewSystem endpoint (%s) for delegated routing, got: %v", expectedNewSystemURL, routerURLs)

	// Should contain ipni.example.com (IPNI is not native)
	foundIPNI := false
	for _, url := range routerURLs {
		if strings.Contains(url, "ipni.example.com") {
			foundIPNI = true
			break
		}
	}
	require.True(t, foundIPNI, "Should contain ipni.example.com endpoint for IPNI")

	// Test passes - we've verified that:
	// 1. Bootstrap peers are correctly resolved from native systems only
	// 2. Delegated routers include both IPNI and NewSystem endpoints
	// 3. URL format is correct (base URLs with paths)
	// 4. AutoConf extensibility works for unknown systems

	t.Log("NewSystem extensibility test passed - Kubo successfully discovered and used unknown routing system")
}
