package autoconf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ma "github.com/multiformats/go-multiaddr"
)

// getLatest fetches the latest config with metadata, using cache when possible
func (c *Client) getLatest(ctx context.Context) (*Response, error) {
	cacheDir, err := c.getCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache dir: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Clean(cacheDir), dirPermOwnerGroupRead); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Check if we have cached data that's still within refreshInterval
	cachedResp, cacheErr := c.getCached(cacheDir)
	if cacheErr == nil && cachedResp.CacheAge < c.refreshInterval {
		log.Debugf("using cached autoconf within refresh interval")
		return cachedResp, nil
	}

	// Cache is stale or doesn't exist, try to fetch from remote
	if cacheErr != nil {
		log.Warnf("no valid cache available: %s", cacheErr)
	} else {
		log.Debugf("cache stale, checking for updates")
	}

	// Select a random URL for load balancing
	configURL := c.selectURL()
	resp, err := c.fetchFromRemote(ctx, configURL, cacheDir)
	if err != nil {
		log.Warnf("failed to fetch from remote: %v", err)
		// Fall back to cached version if available, even if stale
		if cacheErr == nil {
			log.Errorf("using %s-old cached autoconf (last successful fetch: %s)",
				formatDuration(cachedResp.CacheAge), cachedResp.FetchTime.Format(time.RFC3339))
			return cachedResp, nil
		}
		return nil, fmt.Errorf("failed to fetch from remote (%w) and no valid cache available (%w)", err, cacheErr)
	}

	// Successfully fetched new config, now check if server AutoConfTTL requires using cached version
	if resp.Config != nil && cacheErr == nil {
		effectiveInterval := calculateEffectiveRefreshInterval(c.refreshInterval, resp.Config.AutoConfTTL)
		if effectiveInterval < c.refreshInterval {
			log.Debugf("server AutoConfTTL is shorter than user refresh interval, using server AutoConfTTL")

			// Re-check if cached version is still fresh under the effective (shorter) interval
			if cachedResp.CacheAge < effectiveInterval {
				log.Debugf("cached config is still fresh under server AutoConfTTL")
				return cachedResp, nil
			}
			// If cached version is also stale under server TTL, continue with newly fetched config
			log.Debugf("cached config is stale even under server AutoConfTTL, using newly fetched config")
		} else {
			log.Debugf("using user refresh interval, server AutoConfTTL is longer or not specified")
		}
	}

	// Clean up old versions
	if err := c.cleanupOldVersions(cacheDir); err != nil {
		log.Errorf("failed to cleanup old versions: %v", err)
	}

	return resp, nil
}

// GetLatest fetches the latest config with metadata, using cache when possible.
// This is the public API method that returns errors for proper error handling.
func (c *Client) GetLatest(ctx context.Context) (*Response, error) {
	return c.getLatest(ctx)
}

// GetCached returns config from cache or fallback without any network I/O.
//
// This method guarantees no network operations will be performed, making it safe to
// call from any context where network access should be avoided. It only uses locally
// cached data or the configured fallback. It follows this priority order:
//
// 1. Return cached config if available (regardless of age)
// 2. Use configured fallback function (defaults to mainnet fallback)
//
// This method is essential for preventing network blocking during config reads,
// especially in scenarios where autoconf is enabled but network access should
// be avoided (e.g., config validation, offline node construction).
func (c *Client) GetCached() *Config {
	cacheDir, err := c.getCacheDir()
	if err != nil {
		log.Debugf("GetCached: cache dir error: %v, using fallback", err)
		return c.fallbackFunc()
	}

	// Try to get from cache only
	config, err := c.getCachedConfig(cacheDir)
	if err != nil {
		log.Debugf("GetCached: cache miss or error: %v, using fallback", err)
		return c.fallbackFunc()
	}

	log.Debugf("GetCached: returning cached config")
	return config
}

// GetCachedOrRefresh fetches the latest config from network with fallback handling.
//
// This method will attempt to fetch fresh config from the network, respecting the
// refreshInterval for cache freshness. If network fetch fails, it falls back to:
// 1. Cached config (even if stale)
// 2. Configured fallback function (defaults to mainnet fallback)
//
// This method may block on network I/O and should be used when network operations
// are acceptable. For config reads that must avoid network I/O, use GetCached.
func (c *Client) GetCachedOrRefresh(ctx context.Context) *Config {
	resp, err := c.getLatest(ctx)
	if err != nil {
		log.Errorf("AutoConf fetch failed: %v, falling back to fallback config", err)
	} else if resp == nil || resp.Config == nil {
		log.Errorf("AutoConf fetch returned nil response or config, falling back to fallback config")
	} else {
		// Return the fetched config as-is without any modification
		if resp.FromCache() {
			log.Debugf("GetCachedOrRefresh: returning cached config")
		} else {
			log.Debugf("GetCachedOrRefresh: returning fresh config from network")
		}
		return resp.Config
	}

	// Use the configured fallback function
	log.Debugf("GetCachedOrRefresh: returning fallback config")
	return c.fallbackFunc()
}

// HasCachedConfig checks if there's a cached config available.
//
// This method performs a quick filesystem check to determine if cached autoconf
// data exists. It does not validate the cached data or check if it's fresh -
// it only verifies that cache files are present and readable.
//
// Returns:
//   - true: if cached config files exist and are accessible
//   - false: if no cache exists, cache is unreadable, or any error occurs
//
// This method is useful for determining whether to perform cache priming or
// for conditional logic that depends on cache availability.
func (c *Client) HasCachedConfig() bool {
	cacheDir, err := c.getCacheDir()
	if err != nil {
		return false
	}

	// Lock to ensure thread-safe cache reads
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	files, err := c.listCacheFiles(cacheDir)
	return err == nil && len(files) > 0
}

// fetchFromRemote fetches config from remote URL with metadata
func (c *Client) fetchFromRemote(ctx context.Context, configURL, cacheDir string) (*Response, error) {
	// Use local time for fetch time, not HTTP headers
	fetchTime := time.Now()

	resp, config, err := c.fetchFromRemoteRaw(ctx, configURL, cacheDir)
	if err != nil {
		return nil, err
	}

	// Store ETag or Last-Modified for conditional requests
	version := ""
	if etag := resp.Header.Get("ETag"); etag != "" {
		version = etag
	} else if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
		version = lastMod
	}

	return &Response{
		Config:    config,
		FetchTime: fetchTime,
		Version:   version,
		CacheAge:  0,
	}, nil
}

// fetchFromRemoteRaw fetches config from remote URL (internal helper)
func (c *Client) fetchFromRemoteRaw(ctx context.Context, configURL, cacheDir string) (*http.Response, *Config, error) {
	// Validate URL scheme for security
	if err := c.validateURL(configURL); err != nil {
		return nil, nil, fmt.Errorf("invalid config URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", configURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add user agent
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	// Add conditional headers for caching
	etag, lastModified := c.readMetadata(cacheDir)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
		log.Debugf("conditional request to %q with ETag: %q", configURL, etag)
	} else if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
		log.Infof("conditional request to %q with If-Modified-Since: %q", configURL, lastModified)
	} else {
		log.Infof("fetching autoconf from %q", configURL)
	}

	resp, err := c.httpClient.Do(req)
	httpRequestTime := time.Now() // Record actual HTTP request time
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch config: %w", err)
	}
	defer resp.Body.Close()

	// If not modified, record refresh time and return cached version
	if resp.StatusCode == http.StatusNotModified {
		log.Debugf("HTTP 304 Not Modified, updating last refresh time")

		// Record when we last checked for updates (with lock for thread safety)
		timestampStr := httpRequestTime.Format(time.RFC3339)
		c.cacheMu.Lock()
		err = writeOwnerOnlyFile(filepath.Join(cacheDir, lastRefreshFile), []byte(timestampStr))
		c.cacheMu.Unlock()
		if err != nil {
			log.Warnf("failed to write last refresh time: %v", err)
		}

		config, err := c.getCachedConfig(cacheDir)
		return resp, config, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response with size limit
	limitReader := io.LimitReader(resp.Body, c.maxResponseSize)
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log successful HTTP response details
	newEtag := resp.Header.Get("ETag")
	newLastModified := resp.Header.Get("Last-Modified")
	log.Debugf("HTTP %d, got %d bytes, ETag: %q, Last-Modified: %q", resp.StatusCode, len(body), newEtag, newLastModified)

	// Parse JSON
	var config Config
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate config
	if config.AutoConfVersion == 0 {
		return nil, nil, fmt.Errorf("invalid config: missing AutoConfVersion")
	}

	// Validate schema version
	if config.AutoConfSchema == 0 {
		return nil, nil, fmt.Errorf("invalid config: missing AutoConfSchema")
	}
	if config.AutoConfSchema != SupportedAutoConfSchema {
		return nil, nil, fmt.Errorf("unsupported autoconf schema version %d (this client supports version %d, consider updating)", config.AutoConfSchema, SupportedAutoConfSchema)
	}

	// Validate all multiaddr and URL values
	if err := c.validateConfig(&config); err != nil {
		return nil, nil, fmt.Errorf("invalid autoconf JSON: %w", err)
	}

	// Check if this is a duplicate of the latest cached version
	if !c.isNewPayload(cacheDir, body) {
		log.Debugf("received identical payload, skipping cache update")
		return resp, &config, nil
	}

	// Save to cache with metadata and refresh time
	if err := c.saveToCache(cacheDir, body, newEtag, newLastModified, httpRequestTime); err != nil {
		log.Warnf("failed to save to cache: %v", err)
	}

	log.Infof("fetched autoconf version %d", config.AutoConfVersion)
	return resp, &config, nil
}

// getCachedConfig returns the latest cached config
func (c *Client) getCachedConfig(cacheDir string) (*Config, error) {
	// RLock to allow concurrent cache reads while preventing cleanup races
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	files, err := c.listCacheFiles(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no cached versions available")
	}

	// Get the latest file (files are sorted in descending order by timestamp)
	latestFile := files[0]

	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cached config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cached config: %w", err)
	}

	return &config, nil
}

// getCached returns the latest cached config with metadata
func (c *Client) getCached(cacheDir string) (*Response, error) {
	config, err := c.getCachedConfig(cacheDir)
	if err != nil {
		return nil, err
	}

	// Get last HTTP request time to calculate cache age
	lastRefresh, err := c.readLastRefresh(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read last refresh time: %w", err)
	}

	fetchTime := lastRefresh
	cacheAge := time.Since(fetchTime)

	// Get version from cached metadata
	etag, lastModified := c.readMetadata(cacheDir)
	version := ""
	if etag != "" {
		version = etag
	} else if lastModified != "" {
		version = lastModified
	}

	return &Response{
		Config:    config,
		FetchTime: fetchTime,
		Version:   version,
		CacheAge:  cacheAge,
	}, nil
}

// listCacheFiles returns all cached files sorted by timestamp (newest first)
func (c *Client) listCacheFiles(cacheDir string) ([]string, error) {
	cleanCacheDir := filepath.Clean(cacheDir)
	entries, err := os.ReadDir(cleanCacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") && strings.Contains(entry.Name(), "autoconf-") {
			files = append(files, filepath.Join(cleanCacheDir, entry.Name()))
		}
	}

	// Sort by filename (which contains unix timestamp) in descending order
	sort.Slice(files, func(i, j int) bool {
		return filepath.Base(files[i]) > filepath.Base(files[j])
	})

	return files, nil
}

// saveToCache saves config to cache with metadata and refresh time
func (c *Client) saveToCache(cacheDir string, data []byte, etag, lastModified string, httpRequestTime time.Time) error {
	// Lock to ensure thread-safe cache writes
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	cleanCacheDir := filepath.Clean(cacheDir)

	// Use unix timestamp for filename to avoid trusting external values
	timestamp := time.Now().Unix()
	filename := filepath.Join(cleanCacheDir, fmt.Sprintf("autoconf-%d.json", timestamp))

	// Save the main config file
	if err := writeOwnerOnlyFile(filename, data); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Save metadata (ETag and Last-Modified)
	if etag != "" {
		if err := writeOwnerOnlyFile(filepath.Join(cleanCacheDir, etagFile), []byte(etag)); err != nil {
			log.Warnf("failed to write etag: %v", err)
		}
	}
	if lastModified != "" {
		if err := writeOwnerOnlyFile(filepath.Join(cleanCacheDir, lastModifiedFile), []byte(lastModified)); err != nil {
			log.Warnf("failed to write last-modified: %v", err)
		}
	}

	// Save last refresh time
	timestampStr := httpRequestTime.Format(time.RFC3339)
	if err := writeOwnerOnlyFile(filepath.Join(cleanCacheDir, lastRefreshFile), []byte(timestampStr)); err != nil {
		log.Warnf("failed to write last refresh time: %v", err)
	}

	return nil
}

// isNewPayload checks if the payload is different from the latest cached version
func (c *Client) isNewPayload(cacheDir string, newData []byte) bool {
	// RLock to allow concurrent payload comparisons
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	files, err := c.listCacheFiles(cacheDir)
	if err != nil || len(files) == 0 {
		// No cached files, this is new
		return true
	}

	// Read the latest cached file
	latestData, err := os.ReadFile(files[0])
	if err != nil {
		// Error reading cached file, treat as new
		return true
	}

	// Compare payloads
	return !bytes.Equal(latestData, newData)
}

// cleanupOldVersions removes old cached versions beyond maxVersions
func (c *Client) cleanupOldVersions(cacheDir string) error {
	// Use write lock for cleanup operations (exclusive access needed)
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	files, err := c.listCacheFiles(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to get cached files: %w", err)
	}

	if len(files) <= c.cacheSize {
		return nil
	}

	// Remove files beyond cacheSize (keep latest ones)
	for _, file := range files[c.cacheSize:] {
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("failed to remove cache file %s: %w", file, err)
		}
		log.Debugf("removed old cache file: %s", file)
	}

	return nil
}

// validateURL validates that the config URL uses an allowed scheme
func (c *Client) validateURL(configURL string) error {
	u, err := url.Parse(configURL)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	switch strings.ToLower(u.Scheme) {
	case "https":
		// HTTPS is preferred and always allowed
		return nil
	case "http":
		// HTTP is allowed for testing/development
		return nil
	default:
		return fmt.Errorf("unsupported URL scheme '%s' - only HTTP and HTTPS are allowed", u.Scheme)
	}
}

// formatDuration formats a duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	days := d.Hours() / 24
	if days < 7 {
		return fmt.Sprintf("%.1f-day", days)
	}
	weeks := days / 7
	return fmt.Sprintf("%.1f-week", weeks)
}

// validateHTTPURL validates that a URL is an absolute HTTP/HTTPS URL
func (c *Client) validateHTTPURL(urlStr, fieldContext string) error {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%s URL %q invalid: %w", fieldContext, urlStr, err)
	}

	// Require absolute URLs with HTTP/HTTPS scheme
	if parsed.Scheme == "" {
		return fmt.Errorf("%s URL %q must be absolute (missing scheme)", fieldContext, urlStr)
	}

	// Check scheme first before host to provide better error messages
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s URL %q must use http or https scheme, got %q", fieldContext, urlStr, parsed.Scheme)
	}

	// Only check host after confirming valid scheme
	if parsed.Host == "" {
		return fmt.Errorf("%s URL %q must have a host", fieldContext, urlStr)
	}

	return nil
}

// validateConfig validates all multiaddr and URL values in the config
func (c *Client) validateConfig(config *Config) error {
	// Validate SystemRegistry bootstrap multiaddrs
	for systemName, system := range config.SystemRegistry {
		if system.NativeConfig != nil {
			for i, bootstrap := range system.NativeConfig.Bootstrap {
				if _, err := ma.NewMultiaddr(bootstrap); err != nil {
					return fmt.Errorf("SystemRegistry[%q].NativeConfig.Bootstrap[%d] invalid multiaddr %q: %w", systemName, i, bootstrap, err)
				}
			}
		}
	}

	// Validate DNS resolver URLs (must be absolute HTTP/HTTPS URLs like DelegatedEndpoints)
	for tld, resolverURLs := range config.DNSResolvers {
		for i, urlStr := range resolverURLs {
			fieldContext := fmt.Sprintf("DNSResolvers[%q][%d]", tld, i)
			if err := c.validateHTTPURL(urlStr, fieldContext); err != nil {
				return err
			}
		}
	}

	// Validate DelegatedEndpoints URLs (must be absolute HTTP/HTTPS URLs)
	for endpointURL, endpointConfig := range config.DelegatedEndpoints {
		if err := c.validateHTTPURL(endpointURL, "DelegatedEndpoints"); err != nil {
			return err
		}

		// Validate Read paths
		for i, path := range endpointConfig.Read {
			if !strings.HasPrefix(path, "/") {
				return fmt.Errorf("DelegatedEndpoints[%q].Read[%d] path %q must start with /", endpointURL, i, path)
			}
		}

		// Validate Write paths
		for i, path := range endpointConfig.Write {
			if !strings.HasPrefix(path, "/") {
				return fmt.Errorf("DelegatedEndpoints[%q].Write[%d] path %q must start with /", endpointURL, i, path)
			}
		}
	}

	return nil
}

// calculateEffectiveRefreshInterval returns the minimum of user-provided interval and server AutoConfTTL.
// This ensures that both user preferences and server cache policies are respected.
// If cacheTTLSeconds is 0 or negative, only the user interval is used.
func calculateEffectiveRefreshInterval(userInterval time.Duration, cacheTTLSeconds int) time.Duration {
	if cacheTTLSeconds <= 0 {
		// Server doesn't specify TTL or specifies invalid TTL, use user preference
		return userInterval
	}

	serverTTL := time.Duration(cacheTTLSeconds) * time.Second
	return min(serverTTL, userInterval)
}
