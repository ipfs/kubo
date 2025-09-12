package autoconf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/boxo/autoconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAutoConfWithFallback is a helper function that tests autoconf parsing with fallback detection
func testAutoConfWithFallback(t *testing.T, serverURL string, expectError bool, expectErrorMsg string) (*autoconf.Config, bool) {
	return testAutoConfWithFallbackAndTimeout(t, serverURL, expectError, expectErrorMsg, 10*time.Second)
}

// testAutoConfWithFallbackAndTimeout is a helper function that tests autoconf parsing with fallback detection and custom timeout
func testAutoConfWithFallbackAndTimeout(t *testing.T, serverURL string, expectError bool, expectErrorMsg string, timeout time.Duration) (*autoconf.Config, bool) {
	// Use fallback detection to test error conditions with MustGetConfigWithRefresh
	fallbackUsed := false
	fallbackConfig := &autoconf.Config{
		AutoConfVersion: -999, // Special marker to detect fallback usage
		AutoConfSchema:  -999,
	}

	client, err := autoconf.NewClient(
		autoconf.WithUserAgent("test-agent"),
		autoconf.WithURL(serverURL),
		autoconf.WithRefreshInterval(autoconf.DefaultRefreshInterval),
		autoconf.WithFallback(func() *autoconf.Config {
			fallbackUsed = true
			return fallbackConfig
		}),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result := client.GetCachedOrRefresh(ctx)

	if expectError {
		require.True(t, fallbackUsed, expectErrorMsg)
		require.Equal(t, int64(-999), result.AutoConfVersion, "Should return fallback config for error case")
	} else {
		require.False(t, fallbackUsed, "Expected no fallback to be used")
		require.NotEqual(t, int64(-999), result.AutoConfVersion, "Should return fetched config for success case")
	}

	return result, fallbackUsed
}

func TestAutoConfFuzz(t *testing.T) {
	t.Parallel()

	t.Run("fuzz autoconf version", testFuzzAutoConfVersion)
	t.Run("fuzz bootstrap arrays", testFuzzBootstrapArrays)
	t.Run("fuzz dns resolvers", testFuzzDNSResolvers)
	t.Run("fuzz delegated routers", testFuzzDelegatedRouters)
	t.Run("fuzz delegated publishers", testFuzzDelegatedPublishers)
	t.Run("fuzz malformed json", testFuzzMalformedJSON)
	t.Run("fuzz large payloads", testFuzzLargePayloads)
}

func testFuzzAutoConfVersion(t *testing.T) {
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
				"AutoConfVersion": tc.version,
				"AutoConfSchema":  1,
				"AutoConfTTL":     86400,
				"SystemRegistry": map[string]interface{}{
					"AminoDHT": map[string]interface{}{
						"Description": "Test AminoDHT system",
						"NativeConfig": map[string]interface{}{
							"Bootstrap": []string{
								"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
							},
						},
					},
				},
				"DNSResolvers":       map[string]interface{}{},
				"DelegatedEndpoints": map[string]interface{}{},
			}

			jsonData, err := json.Marshal(config)
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonData)
			}))
			defer server.Close()

			// Test that our autoconf parser handles this gracefully
			_, _ = testAutoConfWithFallback(t, server.URL, tc.expectError, fmt.Sprintf("Expected fallback to be used for %s", tc.name))
		})
	}
}

func testFuzzBootstrapArrays(t *testing.T) {
	type testCase struct {
		name        string
		bootstrap   interface{}
		expectError bool
		validate    func(*testing.T, *autoconf.Response)
	}

	testCases := []testCase{
		{
			name:      "valid bootstrap",
			bootstrap: []string{"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"},
			validate: func(t *testing.T, resp *autoconf.Response) {
				expected := []string{"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"}
				bootstrapPeers := resp.Config.GetBootstrapPeers("AminoDHT")
				assert.Equal(t, expected, bootstrapPeers, "Bootstrap peers should match configured values")
			},
		},
		{
			name:      "empty bootstrap",
			bootstrap: []string{},
			validate: func(t *testing.T, resp *autoconf.Response) {
				bootstrapPeers := resp.Config.GetBootstrapPeers("AminoDHT")
				assert.Empty(t, bootstrapPeers, "Empty bootstrap should result in empty peers")
			},
		},
		{
			name:      "null bootstrap",
			bootstrap: nil,
			validate: func(t *testing.T, resp *autoconf.Response) {
				bootstrapPeers := resp.Config.GetBootstrapPeers("AminoDHT")
				assert.Empty(t, bootstrapPeers, "Null bootstrap should result in empty peers")
			},
		},
		{
			name:        "invalid multiaddr",
			bootstrap:   []string{"invalid-multiaddr"},
			expectError: true,
		},
		{
			name:      "very long multiaddr",
			bootstrap: []string{"/dnsaddr/" + strings.Repeat("a", 100) + ".com/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"},
			validate: func(t *testing.T, resp *autoconf.Response) {
				expected := []string{"/dnsaddr/" + strings.Repeat("a", 100) + ".com/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"}
				bootstrapPeers := resp.Config.GetBootstrapPeers("AminoDHT")
				assert.Equal(t, expected, bootstrapPeers, "Very long multiaddr should be preserved")
			},
		},
		{
			name:        "bootstrap as string",
			bootstrap:   "/dnsaddr/test",
			expectError: true,
		},
		{
			name:        "bootstrap as number",
			bootstrap:   123,
			expectError: true,
		},
		{
			name:        "mixed types in array",
			bootstrap:   []interface{}{"/dnsaddr/test", 123, nil},
			expectError: true,
		},
		{
			name:      "extremely large array",
			bootstrap: make([]string, 1000),
			validate: func(t *testing.T, resp *autoconf.Response) {
				// Array will be filled in the loop below
				bootstrapPeers := resp.Config.GetBootstrapPeers("AminoDHT")
				assert.Len(t, bootstrapPeers, 1000, "Large bootstrap array should be preserved")
			},
		},
	}

	// Fill the large array with valid multiaddrs
	largeArray := testCases[len(testCases)-1].bootstrap.([]string)
	for i := range largeArray {
		largeArray[i] = fmt.Sprintf("/dnsaddr/bootstrap%d.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN", i)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"AutoConfVersion": 2025072301,
				"AutoConfSchema":  1,
				"AutoConfTTL":     86400,
				"SystemRegistry": map[string]interface{}{
					"AminoDHT": map[string]interface{}{
						"Description": "Test AminoDHT system",
						"NativeConfig": map[string]interface{}{
							"Bootstrap": tc.bootstrap,
						},
					},
				},
				"DNSResolvers":       map[string]interface{}{},
				"DelegatedEndpoints": map[string]interface{}{},
			}

			jsonData, err := json.Marshal(config)
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonData)
			}))
			defer server.Close()

			autoConf, fallbackUsed := testAutoConfWithFallback(t, server.URL, tc.expectError, fmt.Sprintf("Expected fallback to be used for %s", tc.name))

			if !tc.expectError {
				require.NotNil(t, autoConf, "AutoConf should not be nil for successful parsing")

				// Verify structure is reasonable
				bootstrapPeers := autoConf.GetBootstrapPeers("AminoDHT")
				require.IsType(t, []string{}, bootstrapPeers, "Bootstrap should be []string")

				// Run test-specific validation if provided (only for non-fallback cases)
				if tc.validate != nil && !fallbackUsed {
					// Create a mock Response for compatibility with validation functions
					mockResponse := &autoconf.Response{Config: autoConf}
					tc.validate(t, mockResponse)
				}
			}
		})
	}
}

func testFuzzDNSResolvers(t *testing.T) {
	type testCase struct {
		name        string
		resolvers   interface{}
		expectError bool
		validate    func(*testing.T, *autoconf.Response)
	}

	testCases := []testCase{
		{
			name:      "valid resolvers",
			resolvers: map[string][]string{".": {"https://dns.google/dns-query"}},
			validate: func(t *testing.T, resp *autoconf.Response) {
				expected := map[string][]string{".": {"https://dns.google/dns-query"}}
				assert.Equal(t, expected, resp.Config.DNSResolvers, "DNS resolvers should match configured values")
			},
		},
		{
			name:      "empty resolvers",
			resolvers: map[string][]string{},
			validate: func(t *testing.T, resp *autoconf.Response) {
				assert.Empty(t, resp.Config.DNSResolvers, "Empty resolvers should result in empty map")
			},
		},
		{
			name:      "null resolvers",
			resolvers: nil,
			validate: func(t *testing.T, resp *autoconf.Response) {
				assert.Empty(t, resp.Config.DNSResolvers, "Null resolvers should result in empty map")
			},
		},
		{
			name:        "relative URL (missing scheme)",
			resolvers:   map[string][]string{".": {"not-a-url"}},
			expectError: true, // Should error due to strict HTTP/HTTPS validation
		},
		{
			name:        "invalid URL format",
			resolvers:   map[string][]string{".": {"://invalid-missing-scheme"}},
			expectError: true, // Should error because url.Parse() fails
		},
		{
			name:        "non-HTTP scheme",
			resolvers:   map[string][]string{".": {"ftp://example.com/dns-query"}},
			expectError: true, // Should error due to non-HTTP/HTTPS scheme
		},
		{
			name:      "very long domain",
			resolvers: map[string][]string{strings.Repeat("a", 1000) + ".com": {"https://dns.google/dns-query"}},
			validate: func(t *testing.T, resp *autoconf.Response) {
				expected := map[string][]string{strings.Repeat("a", 1000) + ".com": {"https://dns.google/dns-query"}}
				assert.Equal(t, expected, resp.Config.DNSResolvers, "Very long domain should be preserved")
			},
		},
		{
			name:      "many resolvers",
			resolvers: generateManyResolvers(100),
			validate: func(t *testing.T, resp *autoconf.Response) {
				expected := generateManyResolvers(100)
				assert.Equal(t, expected, resp.Config.DNSResolvers, "Many resolvers should be preserved")
				assert.Equal(t, 100, len(resp.Config.DNSResolvers), "Should have 100 resolvers")
			},
		},
		{
			name:        "resolvers as array",
			resolvers:   []string{"https://dns.google/dns-query"},
			expectError: true,
		},
		{
			name:        "nested invalid structure",
			resolvers:   map[string]interface{}{".": map[string]string{"invalid": "structure"}},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"AutoConfVersion": 2025072301,
				"AutoConfSchema":  1,
				"AutoConfTTL":     86400,
				"SystemRegistry": map[string]interface{}{
					"AminoDHT": map[string]interface{}{
						"Description": "Test AminoDHT system",
						"NativeConfig": map[string]interface{}{
							"Bootstrap": []string{"/dnsaddr/test"},
						},
					},
				},
				"DNSResolvers":       tc.resolvers,
				"DelegatedEndpoints": map[string]interface{}{},
			}

			jsonData, err := json.Marshal(config)
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonData)
			}))
			defer server.Close()

			autoConf, fallbackUsed := testAutoConfWithFallback(t, server.URL, tc.expectError, fmt.Sprintf("Expected fallback to be used for %s", tc.name))

			if !tc.expectError {
				require.NotNil(t, autoConf, "AutoConf should not be nil for successful parsing")

				// Run test-specific validation if provided (only for non-fallback cases)
				if tc.validate != nil && !fallbackUsed {
					// Create a mock Response for compatibility with validation functions
					mockResponse := &autoconf.Response{Config: autoConf}
					tc.validate(t, mockResponse)
				}
			}
		})
	}
}

func testFuzzDelegatedRouters(t *testing.T) {
	// Test various malformed delegated router configurations
	type testCase struct {
		name        string
		routers     interface{}
		expectError bool
		validate    func(*testing.T, *autoconf.Response)
	}

	testCases := []testCase{
		{
			name: "valid endpoints",
			routers: map[string]interface{}{
				"https://ipni.example.com": map[string]interface{}{
					"Systems": []string{"IPNI"},
					"Read":    []string{"/routing/v1/providers"},
					"Write":   []string{},
				},
			},
			validate: func(t *testing.T, resp *autoconf.Response) {
				assert.Len(t, resp.Config.DelegatedEndpoints, 1, "Should have 1 delegated endpoint")
				for url, config := range resp.Config.DelegatedEndpoints {
					assert.Contains(t, url, "ipni.example.com", "Endpoint URL should contain expected domain")
					assert.Contains(t, config.Systems, "IPNI", "Endpoint should have IPNI system")
					assert.Contains(t, config.Read, "/routing/v1/providers", "Endpoint should have providers read path")
				}
			},
		},
		{
			name:    "empty routers",
			routers: map[string]interface{}{},
			validate: func(t *testing.T, resp *autoconf.Response) {
				assert.Empty(t, resp.Config.DelegatedEndpoints, "Empty routers should result in empty endpoints")
			},
		},
		{
			name:    "null routers",
			routers: nil,
			validate: func(t *testing.T, resp *autoconf.Response) {
				assert.Empty(t, resp.Config.DelegatedEndpoints, "Null routers should result in empty endpoints")
			},
		},
		{
			name:        "invalid nested structure",
			routers:     map[string]string{"invalid": "structure"},
			expectError: true,
		},
		{
			name: "invalid endpoint URLs",
			routers: map[string]interface{}{
				"not-a-url": map[string]interface{}{
					"Systems": []string{"IPNI"},
					"Read":    []string{"/routing/v1/providers"},
					"Write":   []string{},
				},
			},
			expectError: true, // Should error due to URL validation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{
				"AutoConfVersion": 2025072301,
				"AutoConfSchema":  1,
				"AutoConfTTL":     86400,
				"SystemRegistry": map[string]interface{}{
					"AminoDHT": map[string]interface{}{
						"Description": "Test AminoDHT system",
						"NativeConfig": map[string]interface{}{
							"Bootstrap": []string{"/dnsaddr/test"},
						},
					},
				},
				"DNSResolvers":       map[string]interface{}{},
				"DelegatedEndpoints": tc.routers,
			}

			jsonData, err := json.Marshal(config)
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonData)
			}))
			defer server.Close()

			autoConf, fallbackUsed := testAutoConfWithFallback(t, server.URL, tc.expectError, fmt.Sprintf("Expected fallback to be used for %s", tc.name))

			if !tc.expectError {
				require.NotNil(t, autoConf, "AutoConf should not be nil for successful parsing")

				// Run test-specific validation if provided (only for non-fallback cases)
				if tc.validate != nil && !fallbackUsed {
					// Create a mock Response for compatibility with validation functions
					mockResponse := &autoconf.Response{Config: autoConf}
					tc.validate(t, mockResponse)
				}
			}
		})
	}
}

func testFuzzDelegatedPublishers(t *testing.T) {
	// DelegatedPublishers use the same autoclient library validation as DelegatedRouters
	// Test that URL validation works for delegated publishers
	type testCase struct {
		name      string
		urls      []string
		expectErr bool
		validate  func(*testing.T, *autoconf.Response)
	}

	testCases := []testCase{
		{
			name: "valid HTTPS URLs",
			urls: []string{"https://delegated-ipfs.dev", "https://another-publisher.com"},
			validate: func(t *testing.T, resp *autoconf.Response) {
				assert.Len(t, resp.Config.DelegatedEndpoints, 2, "Should have 2 delegated endpoints")
				foundURLs := make([]string, 0, len(resp.Config.DelegatedEndpoints))
				for url := range resp.Config.DelegatedEndpoints {
					foundURLs = append(foundURLs, url)
				}
				expectedURLs := []string{"https://delegated-ipfs.dev", "https://another-publisher.com"}
				for _, expectedURL := range expectedURLs {
					assert.Contains(t, foundURLs, expectedURL, "Should contain configured URL: %s", expectedURL)
				}
			},
		},
		{
			name:      "invalid URL",
			urls:      []string{"not-a-url"},
			expectErr: true,
		},
		{
			name: "HTTP URL (accepted during parsing)",
			urls: []string{"http://insecure-publisher.com"},
			validate: func(t *testing.T, resp *autoconf.Response) {
				assert.Len(t, resp.Config.DelegatedEndpoints, 1, "Should have 1 delegated endpoint")
				for url := range resp.Config.DelegatedEndpoints {
					assert.Equal(t, "http://insecure-publisher.com", url, "HTTP URL should be preserved during parsing")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			autoConfData := map[string]interface{}{
				"AutoConfVersion": 2025072301,
				"AutoConfSchema":  1,
				"AutoConfTTL":     86400,
				"SystemRegistry": map[string]interface{}{
					"TestSystem": map[string]interface{}{
						"Description": "Test system for fuzz testing",
						"DelegatedConfig": map[string]interface{}{
							"Read":  []string{"/routing/v1/ipns"},
							"Write": []string{"/routing/v1/ipns"},
						},
					},
				},
				"DNSResolvers":       map[string]interface{}{},
				"DelegatedEndpoints": map[string]interface{}{},
			}

			// Add test URLs as delegated endpoints
			for _, url := range tc.urls {
				autoConfData["DelegatedEndpoints"].(map[string]interface{})[url] = map[string]interface{}{
					"Systems": []string{"TestSystem"},
					"Read":    []string{"/routing/v1/ipns"},
					"Write":   []string{"/routing/v1/ipns"},
				}
			}

			jsonData, err := json.Marshal(autoConfData)
			require.NoError(t, err)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonData)
			}))
			defer server.Close()

			// Test that our autoconf parser handles this gracefully
			autoConf, fallbackUsed := testAutoConfWithFallback(t, server.URL, tc.expectErr, fmt.Sprintf("Expected fallback to be used for %s", tc.name))

			if !tc.expectErr {
				require.NotNil(t, autoConf, "AutoConf should not be nil for successful parsing")

				// Run test-specific validation if provided (only for non-fallback cases)
				if tc.validate != nil && !fallbackUsed {
					// Create a mock Response for compatibility with validation functions
					mockResponse := &autoconf.Response{Config: autoConf}
					tc.validate(t, mockResponse)
				}
			}
		})
	}
}

func testFuzzMalformedJSON(t *testing.T) {
	malformedJSONs := []string{
		`{`,                         // Incomplete JSON
		`{"AutoConfVersion": }`,     // Missing value
		`{"AutoConfVersion": 123,}`, // Trailing comma
		`{AutoConfVersion: 123}`,    // Unquoted key
		`{"Bootstrap": [}`,          // Incomplete array
		`{"Bootstrap": ["/test",]}`, // Trailing comma in array
		`invalid json`,              // Not JSON at all
		`null`,                      // Just null
		`[]`,                        // Array instead of object
		`""`,                        // String instead of object
	}

	for i, malformedJSON := range malformedJSONs {
		t.Run(fmt.Sprintf("malformed_%d", i), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(malformedJSON))
			}))
			defer server.Close()

			// All malformed JSON should result in fallback usage
			_, _ = testAutoConfWithFallback(t, server.URL, true, fmt.Sprintf("Expected fallback to be used for malformed JSON: %s", malformedJSON))
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
		"AutoConfVersion": 2025072301,
		"AutoConfSchema":  1,
		"AutoConfTTL":     86400,
		"SystemRegistry": map[string]interface{}{
			"AminoDHT": map[string]interface{}{
				"Description": "Test AminoDHT system",
				"NativeConfig": map[string]interface{}{
					"Bootstrap": largeBootstrap,
				},
			},
		},
		"DNSResolvers":       largeDNSResolvers,
		"DelegatedEndpoints": map[string]interface{}{},
	}

	jsonData, err := json.Marshal(config)
	require.NoError(t, err)

	t.Logf("Large payload size: %d bytes", len(jsonData))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jsonData)
	}))
	defer server.Close()

	// Should handle large payloads gracefully (up to reasonable limits)
	autoConf, _ := testAutoConfWithFallbackAndTimeout(t, server.URL, false, "Large payload should not trigger fallback", 30*time.Second)
	require.NotNil(t, autoConf, "Should return valid config")

	// Verify bootstrap entries were preserved
	bootstrapPeers := autoConf.GetBootstrapPeers("AminoDHT")
	require.Len(t, bootstrapPeers, 10000, "Should preserve all bootstrap entries")
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
