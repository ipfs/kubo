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

// GetLatestWithMetadata fetches the latest autoconfig with metadata, using cache when possible
func (c *Client) GetLatestWithMetadata(ctx context.Context, configURL string) (*AutoConfigResponse, error) {
	cacheDir, err := c.getCacheDir(configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache dir: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Clean(cacheDir), 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Try to fetch from remote first
	resp, err := c.fetchFromRemoteWithMetadata(ctx, configURL, cacheDir)
	if err != nil {
		log.Warnf("failed to fetch from remote: %v", err)
		// Fall back to cached version
		resp, cacheErr := c.getLatestCachedWithMetadata(cacheDir)
		if cacheErr != nil {
			return nil, fmt.Errorf("failed to fetch from remote (%w) and no valid cache available (%w)", err, cacheErr)
		}
		log.Errorf("using %s-old cached autoconfig (last successful fetch: %s)",
			formatDuration(resp.CacheAge), resp.FetchTime.Format(time.RFC3339))
		return resp, nil
	}

	// Clean up old versions
	if err := c.cleanupOldVersions(cacheDir); err != nil {
		log.Warnf("failed to cleanup old versions: %v", err)
	}

	return resp, nil
}

// GetLatestFromCacheOnly returns the latest cached autoconfig without trying to fetch from remote
func (c *Client) GetLatestFromCacheOnly(cacheDir string) (*AutoConfig, error) {
	return c.getLatestCached(cacheDir)
}

// MustGetConfigWithMainnetFallbacks returns autoconfig with fallbacks to hardcoded defaults
// For cache-only behavior, pass a cancelled context
// This method never returns an error and always returns usable mainnet values
func (c *Client) MustGetConfigWithMainnetFallbacks(ctx context.Context, configURL string) *AutoConfig {
	config, err := c.GetLatest(ctx, configURL)
	if err != nil {
		// Return fallback config
		return &AutoConfig{
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

// fetchFromRemoteWithMetadata fetches config from remote URL with metadata
func (c *Client) fetchFromRemoteWithMetadata(ctx context.Context, configURL, cacheDir string) (*AutoConfigResponse, error) {
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

	return &AutoConfigResponse{
		Config:    config,
		FetchTime: fetchTime,
		Version:   version,
		FromCache: false,
		CacheAge:  0,
	}, nil
}

// fetchFromRemoteRaw fetches config from remote URL (internal helper)
func (c *Client) fetchFromRemoteRaw(ctx context.Context, configURL, cacheDir string) (*http.Response, *AutoConfig, error) {
	// Validate URL scheme for security
	if err := c.validateConfigURL(configURL); err != nil {
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
	etag, lastModified := c.readCachedMetadata(cacheDir)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	} else if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch config: %w", err)
	}
	defer resp.Body.Close()

	// If not modified, return cached version
	if resp.StatusCode == http.StatusNotModified {
		log.Debugf("config not modified, using cached version")
		config, err := c.getLatestCached(cacheDir)
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

	// Parse JSON
	var config AutoConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate config
	if config.AutoConfigVersion == 0 {
		return nil, nil, fmt.Errorf("invalid config: missing AutoConfigVersion")
	}

	// Validate all multiaddr and URL values
	if err := c.validateAutoConfig(&config); err != nil {
		return nil, nil, fmt.Errorf("invalid autoconfig JSON: %w", err)
	}

	// Check if this is a duplicate of the latest cached version
	if !c.isNewPayload(cacheDir, body) {
		log.Debugf("received identical payload, skipping cache update")
		return resp, &config, nil
	}

	// Save to cache with unix timestamp
	if err := c.saveToCacheWithTimestamp(cacheDir, body); err != nil {
		log.Warnf("failed to save to cache: %v", err)
	}

	// Save metadata
	newEtag := resp.Header.Get("ETag")
	newLastModified := resp.Header.Get("Last-Modified")
	if err := c.writeCachedMetadata(cacheDir, newEtag, newLastModified); err != nil {
		log.Warnf("failed to save metadata: %v", err)
	}

	log.Infof("fetched autoconfig version %d", config.AutoConfigVersion)
	return resp, &config, nil
}

// getLatestCached returns the latest cached config
func (c *Client) getLatestCached(cacheDir string) (*AutoConfig, error) {
	files, err := c.getCachedFiles(cacheDir)
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

	var config AutoConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cached config: %w", err)
	}

	return &config, nil
}

// getLatestCachedWithMetadata returns the latest cached config with metadata
func (c *Client) getLatestCachedWithMetadata(cacheDir string) (*AutoConfigResponse, error) {
	config, err := c.getLatestCached(cacheDir)
	if err != nil {
		return nil, err
	}

	// Get cache file modification time to calculate age
	files, err := c.getCachedFiles(cacheDir)
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no cached versions available")
	}

	latestFile := files[0]

	stat, err := os.Stat(latestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to stat cache file: %w", err)
	}

	fetchTime := stat.ModTime()
	cacheAge := time.Since(fetchTime)

	// Get version from cached metadata
	etag, lastModified := c.readCachedMetadata(cacheDir)
	version := ""
	if etag != "" {
		version = etag
	} else if lastModified != "" {
		version = lastModified
	}

	return &AutoConfigResponse{
		Config:    config,
		FetchTime: fetchTime,
		Version:   version,
		FromCache: true,
		CacheAge:  cacheAge,
	}, nil
}

// getCachedFiles returns all cached files sorted by timestamp (newest first)
func (c *Client) getCachedFiles(cacheDir string) ([]string, error) {
	// Sanitize cache directory path
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

// saveToCacheWithTimestamp saves config to cache using unix timestamp
func (c *Client) saveToCacheWithTimestamp(cacheDir string, data []byte) error {
	// Sanitize cache directory path
	cleanCacheDir := filepath.Clean(cacheDir)

	// Use unix timestamp for filename to avoid trusting external values
	timestamp := time.Now().Unix()
	filename := filepath.Join(cleanCacheDir, fmt.Sprintf("autoconfig-%d.json", timestamp))

	// Restrict permissions to owner-only (0600) for security
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	return nil
}

// isNewPayload checks if the payload is different from the latest cached version
func (c *Client) isNewPayload(cacheDir string, newData []byte) bool {
	files, err := c.getCachedFiles(cacheDir)
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
	files, err := c.getCachedFiles(cacheDir)
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

// validateConfigURL validates that the autoconfig URL uses an allowed scheme
func (c *Client) validateConfigURL(configURL string) error {
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

// validateAutoConfig validates all multiaddr and URL values in the autoconfig
func (c *Client) validateAutoConfig(config *AutoConfig) error {
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

// GetLatest fetches the latest autoconfig, using cache when possible (backward compatibility)
func (c *Client) GetLatest(ctx context.Context, configURL string) (*AutoConfig, error) {
	resp, err := c.GetLatestWithMetadata(ctx, configURL)
	if err != nil {
		return nil, err
	}
	return resp.Config, nil
}
