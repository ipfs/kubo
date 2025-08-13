package autoconf

import (
	"crypto/tls"
	"fmt"
	"hash/fnv"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("autoconf")

// writeOwnerOnlyFile writes data to a file with owner-only permissions (0600)
func writeOwnerOnlyFile(filename string, data []byte) error {
	return os.WriteFile(filename, data, filePermOwnerReadWrite)
}

const (
	// DefaultTimeout is the default HTTP timeout for autoconf requests.
	// This timeout balances responsiveness with reliability for most network conditions.
	DefaultTimeout = 5 * time.Second
	// DefaultCacheSize is the default number of cached autoconf versions to keep.
	// Keeping multiple versions provides resilience against corrupted cache files
	// and allows for safe rollback during updates.
	DefaultCacheSize       = 3
	defaultMaxResponseSize = 2 * 1024 * 1024 // 2MiB
	etagFile               = ".etag"
	lastModifiedFile       = ".last-modified"
	lastRefreshFile        = ".last-refresh"

	// File and directory permission constants
	filePermOwnerReadWrite = 0600 // Owner read/write only for sensitive cache files
	dirPermOwnerGroupRead  = 0755 // Owner read/write/execute, group/others read/execute for cache directories

	// DefaultRefreshInterval is the default interval for refreshing autoconf data.
	// This interval strikes a balance between staying up-to-date with network changes
	// and avoiding excessive HTTP requests to the autoconf server. Most IPFS nodes
	// can operate effectively with daily configuration refreshes.
	DefaultRefreshInterval = 24 * time.Hour

	// MainnetAutoConfURL is the default URL for fetching autoconf for the IPFS Mainnet.
	// See https://docs.ipfs.tech/concepts/glossary/#mainnet for more information about IPFS Mainnet.
	MainnetAutoConfURL = "https://conf.ipfs-mainnet.org/autoconf.json"
)

// Client fetches, caches, and manages AutoConf configurations from HTTP endpoints.
//
// The client implements intelligent caching with HTTP conditional requests (ETags/Last-Modified),
// version-based storage, and graceful fallback to cached data when remote servers are unreachable.
// Multiple configuration versions are kept locally to ensure reliability during network issues.
//
// Key features:
//   - HTTP conditional requests to minimize bandwidth
//   - Version-based caching with automatic cleanup
//   - Thread-safe operations with concurrent read support
//   - Graceful fallback to cached data when networks fail
//   - Hostname-based cache separation for multiple URLs
type Client struct {
	httpClient      *http.Client
	cacheDir        string
	cacheSize       int
	userAgent       string
	maxResponseSize int64
	cacheMu         sync.RWMutex   // Protects cache operations (allows concurrent reads)
	urls            []string       // Required: autoconf URLs (can be multiple for load balancing)
	refreshInterval time.Duration  // Required: refresh interval for autoconf data
	fallbackFunc    func() *Config // Optional: fallback function if fetch fails
	updater         *BackgroundUpdater
	updaterMu       sync.Mutex // Protects updater field
}

// Option is a function that configures the client
type Option func(*Client) error

// NewClient creates a new autoconf client with the given options
func NewClient(options ...Option) (*Client, error) {
	c := &Client{
		httpClient:      &http.Client{Timeout: DefaultTimeout},
		cacheSize:       DefaultCacheSize,
		maxResponseSize: defaultMaxResponseSize,
		refreshInterval: DefaultRefreshInterval,   // Default if not specified
		fallbackFunc:    GetMainnetFallbackConfig, // Default fallback
	}

	for _, opt := range options {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Use mainnet URL as default if no URLs provided
	if len(c.urls) == 0 {
		c.urls = []string{MainnetAutoConfURL}
		log.Debugf("no URLs provided, using default mainnet URL: %s", MainnetAutoConfURL)
	}

	// Use temp dir if no cache dir provided
	if c.cacheDir == "" {
		tmpDir, err := os.MkdirTemp("", "ipfs-autoconf-")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		c.cacheDir = tmpDir
		log.Debugf("using temporary cache directory: %s", tmpDir)
	}

	return c, nil
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) error {
		if client == nil {
			return fmt.Errorf("http client cannot be nil")
		}
		c.httpClient = client
		return nil
	}
}

// WithCacheDir sets the cache directory
func WithCacheDir(dir string) Option {
	return func(c *Client) error {
		c.cacheDir = dir
		return nil
	}
}

// WithCacheSize sets the maximum number of versions to keep in cache
func WithCacheSize(size int) Option {
	return func(c *Client) error {
		if size < 1 {
			return fmt.Errorf("cache size must be at least 1")
		}
		c.cacheSize = size
		return nil
	}
}

// WithUserAgent sets the user agent for HTTP requests
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		c.userAgent = ua
		return nil
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		c.httpClient.Timeout = timeout
		return nil
	}
}

// WithTLSInsecureSkipVerify sets whether to skip TLS verification (for testing)
func WithTLSInsecureSkipVerify(skip bool) Option {
	return func(c *Client) error {
		if skip {
			log.Warnf("TLS certificate verification is disabled - this should only be used for testing")
		}
		if c.httpClient.Transport == nil {
			c.httpClient.Transport = http.DefaultTransport.(*http.Transport).Clone()
		}
		if t, ok := c.httpClient.Transport.(*http.Transport); ok {
			if t.TLSClientConfig == nil {
				t.TLSClientConfig = &tls.Config{}
			}
			t.TLSClientConfig.InsecureSkipVerify = skip
		}
		return nil
	}
}

// WithURL adds an autoconf URL to the client.
// Can be called multiple times to add multiple URLs for load balancing.
func WithURL(url string) Option {
	return func(c *Client) error {
		if url == "" {
			return fmt.Errorf("URL cannot be empty")
		}
		// Basic URL validation will happen when used
		c.urls = append(c.urls, url)
		return nil
	}
}

// WithRefreshInterval sets the refresh interval for autoconf data
func WithRefreshInterval(interval time.Duration) Option {
	return func(c *Client) error {
		if interval <= 0 {
			return fmt.Errorf("refresh interval must be positive")
		}
		c.refreshInterval = interval
		return nil
	}
}

// WithFallback sets a fallback function that returns config when fetch fails
func WithFallback(fallbackFunc func() *Config) Option {
	return func(c *Client) error {
		c.fallbackFunc = fallbackFunc
		return nil
	}
}

// getCacheDir returns the cache directory based on the client's URLs
// When multiple URLs are configured, they share the same cache directory
func (c *Client) getCacheDir() (string, error) {
	if len(c.urls) == 0 {
		return "", fmt.Errorf("no URLs configured")
	}

	// Sort URLs for consistent hashing regardless of order
	sortedURLs := make([]string, len(c.urls))
	copy(sortedURLs, c.urls)
	sort.Strings(sortedURLs)

	// Hash all URLs together for a single cache directory
	h := fnv.New64a()
	for _, url := range sortedURLs {
		h.Write([]byte(url))
	}
	hash := h.Sum64()

	// Simple flat structure - just the hash as directory name
	hashStr := fmt.Sprintf("%016x", hash)
	return filepath.Join(c.cacheDir, hashStr), nil
}

// getCacheDirForURL returns the cache directory for a specific URL (for backward compatibility)
func (c *Client) getCacheDirForURL(configURL string) (string, error) {
	// Use FNV-1a for fast, uniform hashing (standard library)
	h := fnv.New64a()
	h.Write([]byte(configURL))
	hash := h.Sum64()

	// Simple flat structure - just the hash as directory name
	hashStr := fmt.Sprintf("%016x", hash)
	return filepath.Join(c.cacheDir, hashStr), nil
}

// selectURL picks a random URL from the configured URLs for load balancing
func (c *Client) selectURL() string {
	if len(c.urls) == 0 {
		return ""
	}
	if len(c.urls) == 1 {
		return c.urls[0]
	}
	return c.urls[rand.Intn(len(c.urls))]
}

// readMetadata reads cached ETag and Last-Modified values
func (c *Client) readMetadata(cacheDir string) (etag, lastModified string) {
	cleanCacheDir := filepath.Clean(cacheDir)

	etagData, etagErr := os.ReadFile(filepath.Join(cleanCacheDir, etagFile))
	lastModData, lastModErr := os.ReadFile(filepath.Join(cleanCacheDir, lastModifiedFile))

	etag = strings.TrimSpace(string(etagData))
	lastModified = strings.TrimSpace(string(lastModData))

	// Only log if both are missing (first request)
	if etagErr != nil && lastModErr != nil {
		log.Debugf("no previous cache metadata found (ETag or Last-Modified)")
	}

	return etag, lastModified
}

// readLastRefresh reads the last HTTP request timestamp from cache
func (c *Client) readLastRefresh(cacheDir string) (time.Time, error) {
	cleanCacheDir := filepath.Clean(cacheDir)

	data, err := os.ReadFile(filepath.Join(cleanCacheDir, lastRefreshFile))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read last refresh time: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse last refresh time: %w", err)
	}

	return timestamp, nil
}
