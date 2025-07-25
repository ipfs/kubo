package autoconfig

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

// GetLatest fetches the latest config with metadata, using cache when possible
// The refreshInterval parameter determines how long cached configs are considered fresh
func (c *Client) GetLatest(ctx context.Context, configURL string, refreshInterval time.Duration) (*Response, error) {
	cacheDir, err := c.getCacheDir(configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache dir: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Clean(cacheDir), 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Check if we have cached data that's still within refreshInterval
	cachedResp, cacheErr := c.getCached(cacheDir)
	if cacheErr == nil && cachedResp.CacheAge < refreshInterval {
		freshFor := refreshInterval - cachedResp.CacheAge
		log.Debugf("using cached autoconfig (age: %s, fresh for %s)", formatDuration(cachedResp.CacheAge), formatDuration(freshFor))
		return cachedResp, nil
	}

	// Cache is stale or doesn't exist, try to fetch from remote
	if cacheErr == nil {
		log.Debugf("cache stale (age: %s), checking for updates", formatDuration(cachedResp.CacheAge))
	}
	resp, err := c.fetchFromRemote(ctx, configURL, cacheDir)
	if err != nil {
		log.Warnf("failed to fetch from remote: %v", err)
		// Fall back to cached version if available, even if stale
		if cacheErr == nil {
			log.Errorf("using %s-old cached autoconfig (last successful fetch: %s)",
				formatDuration(cachedResp.CacheAge), cachedResp.FetchTime.Format(time.RFC3339))
			return cachedResp, nil
		}
		return nil, fmt.Errorf("failed to fetch from remote (%w) and no valid cache available (%w)", err, cacheErr)
	}

	// Clean up old versions
	if err := c.cleanupOldVersions(cacheDir); err != nil {
		log.Warnf("failed to cleanup old versions: %v", err)
	}

	return resp, nil
}

// GetCachedConfig returns the latest cached config without trying to fetch from remote
func (c *Client) GetCachedConfig(cacheDir string) (*Config, error) {
	return c.getCachedConfig(cacheDir)
}

// GetCached returns the latest cached config with metadata without trying to fetch from remote
func (c *Client) GetCached(cacheDir string) (*Response, error) {
	return c.getCached(cacheDir)
}

// MustGetConfig returns config with fallbacks to hardcoded defaults
// For cache-only behavior, pass a cancelled context
// This method never returns an error and always returns usable mainnet values
func (c *Client) MustGetConfig(ctx context.Context, configURL string, refreshInterval time.Duration) *Config {
	resp, err := c.GetLatest(ctx, configURL, refreshInterval)
	var config *Config
	if err == nil {
		config = resp.Config
	}
	if err != nil {
		// Return fallback config
		return &Config{
			Bootstrap:    FallbackBootstrapPeers,
			DNSResolvers: FallbackDNSResolvers,
			DelegatedRouters: map[string]DelegatedRouterConfig{
				MainnetProfileNodesWithDHT:    DelegatedRouterConfig(FallbackDelegatedRouters),
				MainnetProfileNodesWithoutDHT: DelegatedRouterConfig(FallbackDelegatedRouters),
			},
			DelegatedPublishers: map[string]DelegatedPublisherConfig{
				MainnetProfileIPNSPublishers: DelegatedPublisherConfig(FallbackDelegatedPublishers),
			},
		}
	}

	// Fill in missing fields with fallbacks
	if len(config.Bootstrap) == 0 {
		config.Bootstrap = FallbackBootstrapPeers
	}
	if len(config.DNSResolvers) == 0 {
		config.DNSResolvers = FallbackDNSResolvers
	}
	if config.DelegatedRouters == nil {
		config.DelegatedRouters = make(map[string]DelegatedRouterConfig)
	}
	if len(config.DelegatedRouters[MainnetProfileNodesWithDHT]) == 0 {
		config.DelegatedRouters[MainnetProfileNodesWithDHT] = DelegatedRouterConfig(FallbackDelegatedRouters)
	}
	if len(config.DelegatedRouters[MainnetProfileNodesWithoutDHT]) == 0 {
		config.DelegatedRouters[MainnetProfileNodesWithoutDHT] = DelegatedRouterConfig(FallbackDelegatedRouters)
	}
	if config.DelegatedPublishers == nil {
		config.DelegatedPublishers = make(map[string]DelegatedPublisherConfig)
	}
	if len(config.DelegatedPublishers[MainnetProfileIPNSPublishers]) == 0 {
		config.DelegatedPublishers[MainnetProfileIPNSPublishers] = DelegatedPublisherConfig(FallbackDelegatedPublishers)
	}

	return config
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
		FromCache: false,
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
		log.Debugf("conditional request to %q with If-Modified-Since: %q", configURL, lastModified)
	} else {
		log.Debugf("fetching autoconfig from %q", configURL)
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

		// Record when we last checked for updates
		if err := c.writeLastRefresh(cacheDir, httpRequestTime); err != nil {
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
	if config.AutoConfigVersion == 0 {
		return nil, nil, fmt.Errorf("invalid config: missing AutoConfigVersion")
	}

	// Validate all multiaddr and URL values
	if err := c.validateConfig(&config); err != nil {
		return nil, nil, fmt.Errorf("invalid autoconfig JSON: %w", err)
	}

	// Check if this is a duplicate of the latest cached version
	if !c.isNewPayload(cacheDir, body) {
		log.Debugf("received identical payload, skipping cache update")
		return resp, &config, nil
	}

	// Save to cache with unix timestamp
	if err := c.saveToCache(cacheDir, body); err != nil {
		log.Warnf("failed to save to cache: %v", err)
	}

	// Save metadata (reuse already read headers)
	if err := c.writeMetadata(cacheDir, newEtag, newLastModified); err != nil {
		log.Warnf("failed to save metadata: %v", err)
	}

	// Record when we successfully fetched new data
	if err := c.writeLastRefresh(cacheDir, httpRequestTime); err != nil {
		log.Warnf("failed to write last refresh time: %v", err)
	}

	log.Infof("fetched autoconfig version %d", config.AutoConfigVersion)
	return resp, &config, nil
}

// getCachedConfig returns the latest cached config
func (c *Client) getCachedConfig(cacheDir string) (*Config, error) {
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
		FromCache: true,
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
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") && strings.Contains(entry.Name(), "autoconfig-") {
			files = append(files, filepath.Join(cleanCacheDir, entry.Name()))
		}
	}

	// Sort by filename (which contains unix timestamp) in descending order
	sort.Slice(files, func(i, j int) bool {
		return filepath.Base(files[i]) > filepath.Base(files[j])
	})

	return files, nil
}

// saveToCache saves config to cache using unix timestamp
func (c *Client) saveToCache(cacheDir string, data []byte) error {
	cleanCacheDir := filepath.Clean(cacheDir)

	// Use unix timestamp for filename to avoid trusting external values
	timestamp := time.Now().Unix()
	filename := filepath.Join(cleanCacheDir, fmt.Sprintf("autoconfig-%d.json", timestamp))

	if err := writeOwnerOnlyFile(filename, data); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	return nil
}

// isNewPayload checks if the payload is different from the latest cached version
func (c *Client) isNewPayload(cacheDir string, newData []byte) bool {
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
	files, err := c.listCacheFiles(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to get cached files: %w", err)
	}

	if len(files) <= c.cacheSize {
		return nil
	}

	// Remove files beyond cacheSize (keep 3 latest)
	for _, file := range files[c.cacheSize:] {
		if err := os.Remove(file); err != nil {
			log.Warnf("failed to remove old cache file %s: %v", file, err)
		} else {
			log.Debugf("removed old cache file: %s", file)
		}
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

// validateConfig validates all multiaddr and URL values in the config
func (c *Client) validateConfig(config *Config) error {
	// Validate Bootstrap multiaddrs
	for i, bootstrap := range config.Bootstrap {
		if _, err := ma.NewMultiaddr(bootstrap); err != nil {
			return fmt.Errorf("Bootstrap[%d] invalid multiaddr %q: %w", i, bootstrap, err)
		}
	}

	// Validate DNS resolver URLs
	for tld, resolverURL := range config.DNSResolvers {
		for i, urlStr := range resolverURL {
			if _, err := url.Parse(urlStr); err != nil {
				return fmt.Errorf("DNSResolvers[%q][%d] invalid URL %q: %w", tld, i, urlStr, err)
			}
		}
	}

	// Validate DelegatedRouters URLs
	for routerType, routerConfig := range config.DelegatedRouters {
		for i, urlStr := range routerConfig {
			if _, err := url.Parse(urlStr); err != nil {
				return fmt.Errorf("DelegatedRouters[%q][%d] invalid URL %q: %w", routerType, i, urlStr, err)
			}
		}
	}

	// Validate DelegatedPublishers URLs
	for publisherType, publisherConfig := range config.DelegatedPublishers {
		for i, urlStr := range publisherConfig {
			if _, err := url.Parse(urlStr); err != nil {
				return fmt.Errorf("DelegatedPublishers[%q][%d] invalid URL %q: %w", publisherType, i, urlStr, err)
			}
		}
	}

	return nil
}
