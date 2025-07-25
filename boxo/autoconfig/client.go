package autoconfig

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("autoconfig")

const (
	defaultTimeout         = 5 * time.Second
	defaultCacheSize       = 3
	defaultMaxResponseSize = 2 * 1024 * 1024 // 2MiB
	etagFile               = ".etag"
	lastModifiedFile       = ".last-modified"

	// DefaultRefreshInterval is the default interval for refreshing autoconfig data.
	// This interval strikes a balance between staying up-to-date with network changes
	// and avoiding excessive HTTP requests to the autoconfig server. Most IPFS nodes
	// can operate effectively with daily configuration refreshes.
	DefaultRefreshInterval = 24 * time.Hour

	// MainnetAutoConfigURL is the default URL for fetching autoconfig for the IPFS Mainnet.
	// See https://docs.ipfs.tech/concepts/glossary/#mainnet for more information about IPFS Mainnet.
	// This is a specific version that is known to work fine with current implementation,
	// and it makes it a safe default while iterating on format.
	// TODO: change it back to https://config.ipfs-mainnet.org/autoconfig.json before shipping
	MainnetAutoConfigURL = "https://github.com/ipshipyard/config.ipfs-mainnet.org/raw/8fc9d8a793d13922be0fc5ea0634162613eadf6f/autoconfig.json"
)

// Client is the autoconfig client
type Client struct {
	httpClient      *http.Client
	cacheDir        string
	cacheSize       int
	userAgent       string
	maxResponseSize int64
}

// Option is a function that configures the client
type Option func(*Client) error

// NewClient creates a new autoconfig client with the given options
func NewClient(options ...Option) (*Client, error) {
	c := &Client{
		httpClient:      &http.Client{Timeout: defaultTimeout},
		cacheSize:       defaultCacheSize,
		maxResponseSize: defaultMaxResponseSize,
	}

	for _, opt := range options {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Use temp dir if no cache dir provided
	if c.cacheDir == "" {
		tmpDir, err := os.MkdirTemp("", "ipfs-autoconfig-")
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

// sanitizeForPath sanitizes a string to be safe for use in file paths
func sanitizeForPath(s string) string {
	// Replace any character that's not alphanumeric, dash, underscore, or dot
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_\.]`)
	sanitized := re.ReplaceAllString(s, "_")
	// Replace consecutive dots with a single underscore
	re2 := regexp.MustCompile(`\.{2,}`)
	return re2.ReplaceAllString(sanitized, "_")
}

// getCacheDir returns the cache directory for a given URL
func (c *Client) getCacheDir(configURL string) (string, error) {
	u, err := url.Parse(configURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	host := sanitizeForPath(u.Host)
	return filepath.Join(c.cacheDir, "autoconfig", host), nil
}

// readMetadata reads cached ETag and Last-Modified values
func (c *Client) readMetadata(cacheDir string) (etag, lastModified string) {
	// Sanitize cache directory path
	cleanCacheDir := filepath.Clean(cacheDir)

	etagData, err := os.ReadFile(filepath.Join(cleanCacheDir, etagFile))
	if err != nil {
		log.Debugf("previous etag not found: %v", err)
	}

	lastModData, err := os.ReadFile(filepath.Join(cleanCacheDir, lastModifiedFile))
	if err != nil {
		log.Debugf("previous last-modified not found: %v", err)
	}

	return strings.TrimSpace(string(etagData)), strings.TrimSpace(string(lastModData))
}

// writeMetadata writes ETag and Last-Modified values to cache
func (c *Client) writeMetadata(cacheDir, etag, lastModified string) error {
	// Sanitize cache directory path
	cleanCacheDir := filepath.Clean(cacheDir)
	if etag != "" {
		// Use owner-only permissions (0600) for security
		if err := os.WriteFile(filepath.Join(cleanCacheDir, etagFile), []byte(etag), 0600); err != nil {
			return fmt.Errorf("failed to write etag: %w", err)
		}
	}
	if lastModified != "" {
		// Use owner-only permissions (0600) for security
		if err := os.WriteFile(filepath.Join(cleanCacheDir, lastModifiedFile), []byte(lastModified), 0600); err != nil {
			return fmt.Errorf("failed to write last-modified: %w", err)
		}
	}
	return nil
}
