package autoconf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfig(t *testing.T) {
	client := &Client{}

	t.Run("valid config passes validation", func(t *testing.T) {
		config := &Config{
			AutoConfVersion: 123,
			AutoConfSchema:  1,
			SystemRegistry: map[string]SystemConfig{
				SystemAminoDHT: {
					Description: "Test AminoDHT system",
					NativeConfig: &NativeConfig{
						Bootstrap: []string{
							"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
							"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTest",
						},
					},
					DelegatedConfig: &DelegatedConfig{
						Read:  []string{"/routing/v1/providers"},
						Write: []string{},
					},
				},
			},
			DNSResolvers: map[string][]string{
				"eth.": {"https://dns.example.com/dns-query"},
				"foo.": {"http://localhost:8080/dns-query", "https://1.2.3.4/dns-query"},
			},
			DelegatedEndpoints: map[string]EndpointConfig{
				"https://ipni.example.com": {
					Systems: []string{SystemIPNI},
					Read:    []string{"/routing/v1/providers"},
					Write:   []string{},
				},
				"https://delegated-ipfs.dev": {
					Systems: []string{SystemAminoDHT},
					Read:    []string{"/routing/v1/ipns"},
					Write:   []string{"/routing/v1/ipns"},
				},
			},
		}

		err := client.validateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("invalid bootstrap multiaddr fails validation", func(t *testing.T) {
		config := &Config{
			AutoConfVersion: 123,
			AutoConfSchema:  1,
			SystemRegistry: map[string]SystemConfig{
				SystemAminoDHT: {
					NativeConfig: &NativeConfig{
						Bootstrap: []string{
							"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
							"invalid-multiaddr",
						},
					},
				},
			},
		}

		err := client.validateConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SystemRegistry[\"AminoDHT\"].NativeConfig.Bootstrap[1] invalid multiaddr")
		assert.Contains(t, err.Error(), "invalid-multiaddr")
	})

	t.Run("invalid DNS resolver URL fails validation", func(t *testing.T) {
		config := &Config{
			AutoConfVersion: 123,
			AutoConfSchema:  1,
			DNSResolvers: map[string][]string{
				"eth.": {"https://valid.example.com"},
				"bad.": {"://invalid-url"},
			},
		}

		err := client.validateConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DNSResolvers[\"bad.\"][0] URL")
		assert.Contains(t, err.Error(), "://invalid-url")
	})

	t.Run("invalid delegated endpoint URL fails validation", func(t *testing.T) {
		config := &Config{
			AutoConfVersion: 123,
			AutoConfSchema:  1,
			DelegatedEndpoints: map[string]EndpointConfig{
				"://invalid-missing-scheme": {
					Systems: []string{SystemIPNI},
					Read:    []string{"/routing/v1/providers"},
					Write:   []string{},
				},
			},
		}

		err := client.validateConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DelegatedEndpoints URL \"://invalid-missing-scheme\" invalid")
	})

	t.Run("invalid delegated endpoint path fails validation", func(t *testing.T) {
		config := &Config{
			AutoConfVersion: 123,
			AutoConfSchema:  1,
			DelegatedEndpoints: map[string]EndpointConfig{
				"https://valid.example.com": {
					Systems: []string{SystemIPNI},
					Read:    []string{"valid-path", "routing/v1/providers"}, // Missing leading slash
					Write:   []string{},
				},
			},
		}

		err := client.validateConfig(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DelegatedEndpoints[\"https://valid.example.com\"].Read[0] path \"valid-path\" must start with /")
	})

	t.Run("empty config passes validation", func(t *testing.T) {
		config := &Config{
			AutoConfVersion: 123,
			AutoConfSchema:  1,
		}

		err := client.validateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("various valid URL schemes are accepted", func(t *testing.T) {
		config := &Config{
			AutoConfVersion: 123,
			AutoConfSchema:  1,
			DNSResolvers: map[string][]string{
				"test.": {
					"https://example.com",
					"http://localhost:8080",
					"http://192.168.1.1:9090",
					"https://1.2.3.4:443/path",
					"http://[::1]:8080/dns-query", // IPv6
				},
			},
		}

		err := client.validateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("DNS resolvers must use HTTP/HTTPS", func(t *testing.T) {
		testCases := []struct {
			name        string
			url         string
			errContains string
		}{
			{
				name:        "relative URL",
				url:         "not-a-url",
				errContains: "must be absolute (missing scheme)",
			},
			{
				name:        "FTP URL",
				url:         "ftp://example.com/dns-query",
				errContains: "must use http or https scheme, got \"ftp\"",
			},
			{
				name:        "missing host",
				url:         "https:///dns-query",
				errContains: "must have a host",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := &Config{
					AutoConfVersion: 123,
					AutoConfSchema:  1,
					DNSResolvers: map[string][]string{
						"test.": {tc.url},
					},
				}

				err := client.validateConfig(config)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			})
		}
	})

	t.Run("delegated endpoints must use HTTP/HTTPS", func(t *testing.T) {
		testCases := []struct {
			name        string
			url         string
			errContains string
		}{
			{
				name:        "relative URL",
				url:         "ipni.example.com",
				errContains: "must be absolute (missing scheme)",
			},
			{
				name:        "ws URL",
				url:         "ws://example.com/socket",
				errContains: "must use http or https scheme, got \"ws\"",
			},
			{
				name:        "empty URL",
				url:         "",
				errContains: "must be absolute (missing scheme)",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := &Config{
					AutoConfVersion: 123,
					AutoConfSchema:  1,
					DelegatedEndpoints: map[string]EndpointConfig{
						tc.url: {
							Systems: []string{SystemIPNI},
							Read:    []string{"/routing/v1/providers"},
							Write:   []string{},
						},
					},
				}

				err := client.validateConfig(config)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			})
		}
	})
}

func TestValidateHTTPURL(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name         string
		url          string
		fieldContext string
		wantErr      bool
		errContains  string
	}{
		// Valid URLs
		{
			name:         "valid HTTPS URL",
			url:          "https://example.com/dns-query",
			fieldContext: "test",
			wantErr:      false,
		},
		{
			name:         "valid HTTP URL",
			url:          "http://localhost:8080/dns-query",
			fieldContext: "test",
			wantErr:      false,
		},
		{
			name:         "valid HTTPS with IP",
			url:          "https://192.168.1.1:443/path",
			fieldContext: "test",
			wantErr:      false,
		},
		{
			name:         "valid HTTP with IPv6",
			url:          "http://[::1]:8080/dns-query",
			fieldContext: "test",
			wantErr:      false,
		},
		{
			name:         "valid HTTPS with port",
			url:          "https://example.com:8443/dns-query",
			fieldContext: "test",
			wantErr:      false,
		},
		{
			name:         "valid HTTP with query params",
			url:          "http://example.com/dns-query?format=json",
			fieldContext: "test",
			wantErr:      false,
		},
		{
			name:         "valid HTTPS with fragment",
			url:          "https://example.com/dns-query#section",
			fieldContext: "test",
			wantErr:      false,
		},
		{
			name:         "valid HTTP with auth",
			url:          "http://user:pass@example.com/dns-query",
			fieldContext: "test",
			wantErr:      false,
		},
		// Invalid URLs - parsing errors
		{
			name:         "invalid URL format",
			url:          "://invalid-missing-scheme",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL \"://invalid-missing-scheme\" invalid:",
		},
		{
			name:         "URL with spaces",
			url:          "https://exam ple.com/dns-query",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL \"https://exam ple.com/dns-query\" invalid:",
		},
		{
			name:         "URL with newlines",
			url:          "https://example.com\n/dns-query",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL",
		},
		// Invalid URLs - missing components
		{
			name:         "missing scheme (relative URL)",
			url:          "example.com/dns-query",
			fieldContext: "DelegatedEndpoint",
			wantErr:      true,
			errContains:  "DelegatedEndpoint URL \"example.com/dns-query\" must be absolute (missing scheme)",
		},
		{
			name:         "missing scheme (path only)",
			url:          "/dns-query",
			fieldContext: "DelegatedEndpoint",
			wantErr:      true,
			errContains:  "DelegatedEndpoint URL \"/dns-query\" must be absolute (missing scheme)",
		},
		{
			name:         "missing host",
			url:          "https:///dns-query",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL \"https:///dns-query\" must have a host",
		},
		{
			name:         "empty URL",
			url:          "",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL \"\" must be absolute (missing scheme)",
		},
		// Invalid URLs - wrong scheme
		{
			name:         "FTP scheme",
			url:          "ftp://example.com/file",
			fieldContext: "DelegatedEndpoint",
			wantErr:      true,
			errContains:  "DelegatedEndpoint URL \"ftp://example.com/file\" must use http or https scheme, got \"ftp\"",
		},
		{
			name:         "file scheme",
			url:          "file:///etc/hosts",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL \"file:///etc/hosts\" must use http or https scheme, got \"file\"",
		},
		{
			name:         "custom scheme",
			url:          "ipfs://bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL \"ipfs://bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi\" must use http or https scheme, got \"ipfs\"",
		},
		{
			name:         "data URL",
			url:          "data:text/plain;base64,SGVsbG8sIFdvcmxkIQ==",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL \"data:text/plain;base64,SGVsbG8sIFdvcmxkIQ==\" must use http or https scheme, got \"data\"",
		},
		{
			name:         "ws scheme",
			url:          "ws://example.com/socket",
			fieldContext: "DelegatedEndpoint",
			wantErr:      true,
			errContains:  "DelegatedEndpoint URL \"ws://example.com/socket\" must use http or https scheme, got \"ws\"",
		},
		{
			name:         "mixed case scheme",
			url:          "HtTpS://example.com/dns-query",
			fieldContext: "DNSResolver",
			wantErr:      false, // url.Parse normalizes scheme to lowercase
		},
		// Edge cases
		{
			name:         "localhost without scheme",
			url:          "localhost:8080/dns-query",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "must use http or https scheme, got \"localhost\"", // url.Parse treats this as scheme "localhost"
		},
		{
			name:         "IP without scheme",
			url:          "192.168.1.1:8080/dns-query",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL \"192.168.1.1:8080/dns-query\" invalid:", // url.Parse fails on this
		},
		{
			name:         "just domain",
			url:          "example.com",
			fieldContext: "DelegatedEndpoint",
			wantErr:      true,
			errContains:  "DelegatedEndpoint URL \"example.com\" must be absolute (missing scheme)",
		},
		{
			name:         "URL with only scheme",
			url:          "https://",
			fieldContext: "DNSResolver",
			wantErr:      true,
			errContains:  "DNSResolver URL \"https://\" must have a host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateHTTPURL(tt.url, tt.fieldContext)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHTTPCachingBehavior(t *testing.T) {
	t.Parallel()

	// Create test autoconf data
	testConfig := &Config{
		AutoConfVersion: 2025080101,
		AutoConfSchema:  1,
		AutoConfTTL:     3600,
		SystemRegistry: map[string]SystemConfig{
			SystemAminoDHT: {
				Description: "Test AminoDHT system",
				NativeConfig: &NativeConfig{
					Bootstrap: []string{
						"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
					},
				},
			},
		},
	}

	configJSON, err := json.Marshal(testConfig)
	require.NoError(t, err)

	etag := `"test-etag-123"`
	lastModified := "Wed, 21 Oct 2015 07:28:00 GMT"
	var requestCount int32
	var conditionalRequestCount int32

	// Create server that tracks conditional requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		t.Logf("HTTP caching test request #%d: %s, If-None-Match: %s, If-Modified-Since: %s",
			count, r.Method, r.Header.Get("If-None-Match"), r.Header.Get("If-Modified-Since"))

		// Check for conditional request headers
		ifNoneMatch := r.Header.Get("If-None-Match")
		ifModifiedSince := r.Header.Get("If-Modified-Since")

		if ifNoneMatch == etag || ifModifiedSince == lastModified {
			atomic.AddInt32(&conditionalRequestCount, 1)
			// Return 304 Not Modified
			t.Logf("Returning 304 Not Modified for conditional request")
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// Return full response with caching headers
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", lastModified)
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write(configJSON)
	}))
	defer server.Close()

	// Create temporary cache directory
	cacheDir := t.TempDir()

	// Create autoconf client
	fallbackFunc := func() *Config {
		return &Config{
			AutoConfVersion: 1,
			AutoConfSchema:  1,
		}
	}

	client, err := NewClient(
		WithCacheDir(cacheDir),
		WithUserAgent("test-user-agent"),
		WithCacheSize(3),
		WithTimeout(5*time.Second),
		WithURL(server.URL),
		WithRefreshInterval(100*time.Millisecond),
		WithFallback(fallbackFunc),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// First request - should fetch fresh data
	config1 := client.GetCachedOrRefresh(ctx)
	require.NotNil(t, config1)
	assert.Equal(t, int64(2025080101), config1.AutoConfVersion)

	initialRequestCount := atomic.LoadInt32(&requestCount)
	require.GreaterOrEqual(t, int(initialRequestCount), 1, "Should have made at least one initial request")

	// Reset counters to track only subsequent requests
	atomic.StoreInt32(&requestCount, 0)
	atomic.StoreInt32(&conditionalRequestCount, 0)

	// Wait to ensure cache is considered stale (100ms refresh interval)
	time.Sleep(150 * time.Millisecond)

	// Second request - should make conditional request and get 304
	config2 := client.GetCachedOrRefresh(ctx)
	require.NotNil(t, config2)
	assert.Equal(t, int64(2025080101), config2.AutoConfVersion)

	// Wait for request to complete
	time.Sleep(100 * time.Millisecond)

	// Should have made at least one conditional request
	finalRequestCount := atomic.LoadInt32(&requestCount)
	finalConditionalCount := atomic.LoadInt32(&conditionalRequestCount)

	t.Logf("Final request count: %d, conditional count: %d", finalRequestCount, finalConditionalCount)

	assert.GreaterOrEqual(t, int(finalRequestCount), 1, "Should have made at least one request for refresh")
	assert.GreaterOrEqual(t, int(finalConditionalCount), 1, "Should have made at least one conditional request that returned 304")

	// All refresh requests should have been conditional (had If-None-Match or If-Modified-Since header)
	assert.Equal(t, finalConditionalCount, finalRequestCount, "All refresh requests should have been conditional")
}

func TestCalculateEffectiveRefreshInterval(t *testing.T) {
	const (
		// Test-specific constants for TTL values
		testAutoConfTTLOneHour = 3600  // 1 hour in seconds
		testAutoConfTTL12Hours = 43200 // 12 hours in seconds
		testAutoConfTTL24Hours = 86400 // 24 hours in seconds
	)

	tests := []struct {
		name           string
		userInterval   time.Duration
		cacheTTL       int
		expectedResult time.Duration
		description    string
	}{
		{
			name:           "server TTL shorter than user interval",
			userInterval:   24 * time.Hour,
			cacheTTL:       testAutoConfTTLOneHour,
			expectedResult: 1 * time.Hour,
			description:    "should use server TTL when it's shorter",
		},
		{
			name:           "server TTL longer than user interval",
			userInterval:   1 * time.Hour,
			cacheTTL:       testAutoConfTTL24Hours,
			expectedResult: 1 * time.Hour,
			description:    "should use user interval when it's shorter",
		},
		{
			name:           "server TTL equal to user interval",
			userInterval:   12 * time.Hour,
			cacheTTL:       testAutoConfTTL12Hours,
			expectedResult: 12 * time.Hour,
			description:    "should use user interval when equal",
		},
		{
			name:           "server TTL zero",
			userInterval:   6 * time.Hour,
			cacheTTL:       0,
			expectedResult: 6 * time.Hour,
			description:    "should use user interval when server TTL is zero",
		},
		{
			name:           "server TTL negative",
			userInterval:   8 * time.Hour,
			cacheTTL:       -100,
			expectedResult: 8 * time.Hour,
			description:    "should use user interval when server TTL is negative",
		},
		{
			name:           "very short server TTL",
			userInterval:   1 * time.Hour,
			cacheTTL:       60, // 1 minute
			expectedResult: 1 * time.Minute,
			description:    "should handle very short server TTL",
		},
		{
			name:           "very long user interval",
			userInterval:   168 * time.Hour, // 1 week
			cacheTTL:       86400,           // 1 day
			expectedResult: 24 * time.Hour,
			description:    "should use server TTL when user interval is very long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateEffectiveRefreshInterval(tt.userInterval, tt.cacheTTL)
			assert.Equal(t, tt.expectedResult, result, tt.description)

			// Verify the result is always the minimum of the two values (when server TTL > 0)
			if tt.cacheTTL > 0 {
				serverTTL := time.Duration(tt.cacheTTL) * time.Second
				expectedMin := tt.userInterval
				if serverTTL < tt.userInterval {
					expectedMin = serverTTL
				}
				assert.Equal(t, expectedMin, result, "result should be minimum of user interval and server TTL")
			} else {
				// When server TTL <= 0, should always return user interval
				assert.Equal(t, tt.userInterval, result, "should return user interval when server TTL is invalid")
			}
		})
	}
}
