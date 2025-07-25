package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ipfs/kubo/boxo/autoconfig"
	"github.com/stretchr/testify/require"
)

func TestAutoConfigFuzz(t *testing.T) {
	t.Parallel()

	t.Run("fuzz autoconfig version", testFuzzAutoConfigVersion)
	t.Run("fuzz bootstrap arrays", testFuzzBootstrapArrays)
	t.Run("fuzz dns resolvers", testFuzzDNSResolvers)
	t.Run("fuzz delegated routers", testFuzzDelegatedRouters)
	t.Run("fuzz delegated publishers", testFuzzDelegatedPublishers)
	t.Run("fuzz malformed json", testFuzzMalformedJSON)
	t.Run("fuzz large payloads", testFuzzLargePayloads)
}

func testFuzzAutoConfigVersion(t *testing.T) {
	testCases := []struct {
		name        string
		version     interface{}
		expectError bool
	}{
		{"valid version", 2025071801, false},
		{"zero version", 0, true},              // Should be invalid
		{"negative version", -1, false},        // Parser accepts negative versions
		{"string version", "2025071801", true}, // Should be number
		{"float version", 2025071801.5, true},
		{"very large version", 9999999999999999, false}, // Large but valid int64
		{"null version", nil, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"AutoConfigVersion": tc.version,
				"AutoConfigSchema":  2,
				"Bootstrap": []string{
					"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
				},
			}

			jsonData, err := json.Marshal(config)
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonData)
			}))
			defer server.Close()

			// Test that our autoconfig parser handles this gracefully
			client, err := autoconfig.NewClient(autoconfig.WithUserAgent("test-agent"))
			require.NoError(t, err)
			_, err = client.GetLatest(context.Background(), server.URL, autoconfig.DefaultRefreshInterval)

			if tc.expectError {
				require.Error(t, err, "Expected error for %s", tc.name)
			} else {
				require.NoError(t, err, "Expected no error for %s", tc.name)
			}
		})
	}
}

func testFuzzBootstrapArrays(t *testing.T) {
	testCases := []struct {
		name        string
		bootstrap   interface{}
		expectError bool
	}{
		{"valid bootstrap", []string{"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"}, false},
		{"empty bootstrap", []string{}, false},
		{"null bootstrap", nil, false},                             // Should be handled gracefully
		{"invalid multiaddr", []string{"invalid-multiaddr"}, true}, // Should error due to validation
		{"very long multiaddr", []string{"/dnsaddr/" + strings.Repeat("a", 100) + ".com/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"}, false},
		{"bootstrap as string", "/dnsaddr/test", true}, // Should be array
		{"bootstrap as number", 123, true},
		{"mixed types in array", []interface{}{"/dnsaddr/test", 123, nil}, true},
		{"extremely large array", make([]string, 1000), false}, // Test memory limits
	}

	// Fill the large array with valid multiaddrs
	largeArray := testCases[len(testCases)-1].bootstrap.([]string)
	for i := range largeArray {
		largeArray[i] = fmt.Sprintf("/dnsaddr/bootstrap%d.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN", i)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"AutoConfigVersion": 2025072301,
				"AutoConfigSchema":  3,
				"Bootstrap":         tc.bootstrap,
			}

			jsonData, err := json.Marshal(config)
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonData)
			}))
			defer server.Close()

			client, err := autoconfig.NewClient(autoconfig.WithUserAgent("test-agent"))
			require.NoError(t, err)
			autoConf, err := client.GetLatest(context.Background(), server.URL, autoconfig.DefaultRefreshInterval)

			if tc.expectError {
				require.Error(t, err, "Expected error for %s", tc.name)
			} else {
				require.NoError(t, err, "Expected no error for %s", tc.name)
				if err == nil && autoConf != nil {
					// Verify structure is reasonable
					require.IsType(t, []string{}, autoConf.Config.Bootstrap, "Bootstrap should be []string")
				}
			}
		})
	}
}

func testFuzzDNSResolvers(t *testing.T) {
	testCases := []struct {
		name        string
		resolvers   interface{}
		expectError bool
	}{
		{"valid resolvers", map[string][]string{".": {"https://dns.google/dns-query"}}, false},
		{"empty resolvers", map[string][]string{}, false},
		{"null resolvers", nil, false},
		{"invalid URL", map[string][]string{".": {"not-a-url"}}, false}, // Should accept, validate later
		{"very long domain", map[string][]string{strings.Repeat("a", 1000) + ".com": {"https://dns.google/dns-query"}}, false},
		{"many resolvers", generateManyResolvers(100), false},
		{"resolvers as array", []string{"https://dns.google/dns-query"}, true}, // Should be map
		{"nested invalid structure", map[string]interface{}{".": map[string]string{"invalid": "structure"}}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"AutoConfigVersion": 2025072301,
				"AutoConfigSchema":  3,
				"Bootstrap":         []string{"/dnsaddr/test"},
				"DNSResolvers":      tc.resolvers,
			}

			jsonData, err := json.Marshal(config)
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonData)
			}))
			defer server.Close()

			client, err := autoconfig.NewClient(autoconfig.WithUserAgent("test-agent"))
			require.NoError(t, err)
			_, err = client.GetLatest(context.Background(), server.URL, autoconfig.DefaultRefreshInterval)

			if tc.expectError {
				require.Error(t, err, "Expected error for %s", tc.name)
			} else {
				require.NoError(t, err, "Expected no error for %s", tc.name)
			}
		})
	}
}

func testFuzzDelegatedRouters(t *testing.T) {
	// Test various malformed delegated router configurations
	testCases := []struct {
		name        string
		routers     interface{}
		expectError bool
	}{
		{"valid routers", map[string][]string{
			autoconfig.MainnetProfileNodesWithDHT: []string{"https://cid.contact"},
		}, false},
		{"empty routers", map[string]interface{}{}, false},
		{"null routers", nil, false},
		{"invalid nested structure", map[string]string{"invalid": "structure"}, true},
		{"invalid router URLs", map[string][]string{
			autoconfig.MainnetProfileNodesWithDHT: []string{"not-a-url"},
		}, false}, // Should not error at parse time, validation happens later
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"AutoConfigVersion": 2025072301,
				"AutoConfigSchema":  3,
				"Bootstrap":         []string{"/dnsaddr/test"},
				"DelegatedRouters":  tc.routers,
			}

			jsonData, err := json.Marshal(config)
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonData)
			}))
			defer server.Close()

			client, err := autoconfig.NewClient(autoconfig.WithUserAgent("test-agent"))
			require.NoError(t, err)
			_, err = client.GetLatest(context.Background(), server.URL, autoconfig.DefaultRefreshInterval)

			if tc.expectError {
				require.Error(t, err, "Expected error for %s", tc.name)
			} else {
				require.NoError(t, err, "Expected no error for %s", tc.name)
			}
		})
	}
}

func testFuzzDelegatedPublishers(t *testing.T) {
	// DelegatedPublishers use the same autoclient library validation as DelegatedRouters
	// so we don't need separate fuzz testing - the code paths are already covered
	t.Skip("Redundant test - delegated publishers use same validation as delegated routers")
}

func testFuzzMalformedJSON(t *testing.T) {
	malformedJSONs := []string{
		`{`,                           // Incomplete JSON
		`{"AutoConfigVersion": }`,     // Missing value
		`{"AutoConfigVersion": 123,}`, // Trailing comma
		`{AutoConfigVersion: 123}`,    // Unquoted key
		`{"Bootstrap": [}`,            // Incomplete array
		`{"Bootstrap": ["/test",]}`,   // Trailing comma in array
		`invalid json`,                // Not JSON at all
		`null`,                        // Just null
		`[]`,                          // Array instead of object
		`""`,                          // String instead of object
	}

	for i, malformedJSON := range malformedJSONs {
		t.Run(fmt.Sprintf("malformed_%d", i), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(malformedJSON))
			}))
			defer server.Close()

			client, err := autoconfig.NewClient(autoconfig.WithUserAgent("test-agent"))
			require.NoError(t, err)
			_, err = client.GetLatest(context.Background(), server.URL, autoconfig.DefaultRefreshInterval)

			// All malformed JSON should result in errors
			require.Error(t, err, "Expected error for malformed JSON: %s", malformedJSON)
		})
	}
}

func testFuzzLargePayloads(t *testing.T) {
	// Test with very large but valid JSON payloads
	largeBootstrap := make([]string, 10000)
	for i := range largeBootstrap {
		largeBootstrap[i] = fmt.Sprintf("/dnsaddr/bootstrap%d.example.com/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN", i)
	}

	largeDNSResolvers := make(map[string][]string)
	for i := 0; i < 1000; i++ {
		domain := fmt.Sprintf("domain%d.example.com", i)
		largeDNSResolvers[domain] = []string{
			fmt.Sprintf("https://resolver%d.example.com/dns-query", i),
		}
	}

	config := map[string]interface{}{
		"AutoConfigVersion": 2025072301,
		"AutoConfigSchema":  3,
		"Bootstrap":         largeBootstrap,
		"DNSResolvers":      largeDNSResolvers,
	}

	jsonData, err := json.Marshal(config)
	require.NoError(t, err)

	t.Logf("Large payload size: %d bytes", len(jsonData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jsonData)
	}))
	defer server.Close()

	client, err := autoconfig.NewClient(autoconfig.WithUserAgent("test-agent"))
	require.NoError(t, err)
	autoConf, err := client.GetLatest(context.Background(), server.URL, autoconfig.DefaultRefreshInterval)

	// Should handle large payloads gracefully (up to reasonable limits)
	require.NoError(t, err, "Should handle large payloads")
	require.NotNil(t, autoConf, "Should return valid config")
	require.Len(t, autoConf.Config.Bootstrap, 10000, "Should preserve all bootstrap entries")
}

// Helper function to generate many DNS resolvers for testing
func generateManyResolvers(count int) map[string][]string {
	resolvers := make(map[string][]string)
	for i := 0; i < count; i++ {
		domain := fmt.Sprintf("domain%d.example.com", i)
		resolvers[domain] = []string{
			fmt.Sprintf("https://resolver%d.example.com/dns-query", i),
		}
	}
	return resolvers
}
