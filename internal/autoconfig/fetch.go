package autoconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// GetLatest fetches the latest autoconfig, using cache when possible
func (c *Client) GetLatest(ctx context.Context, configURL string) (*AutoConfig, error) {
	cacheDir, err := c.getCacheDir(configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache dir: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Try to fetch from remote first
	config, err := c.fetchFromRemote(ctx, configURL, cacheDir)
	if err != nil {
		log.Warnf("failed to fetch from remote: %v", err)
		// Fall back to cached version
		config, cacheErr := c.getLatestCached(cacheDir)
		if cacheErr != nil {
			return nil, fmt.Errorf("failed to fetch from remote (%w) and no valid cache available (%w)", err, cacheErr)
		}
		log.Infof("using cached autoconfig version %d", config.AutoConfigVersion)
		return config, nil
	}

	// Clean up old versions
	if err := c.cleanupOldVersions(cacheDir); err != nil {
		log.Warnf("failed to cleanup old versions: %v", err)
	}

	return config, nil
}

// fetchFromRemote fetches config from remote URL
func (c *Client) fetchFromRemote(ctx context.Context, configURL, cacheDir string) (*AutoConfig, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", configURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
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
		return nil, fmt.Errorf("failed to fetch config: %w", err)
	}
	defer resp.Body.Close()

	// If not modified, return cached version
	if resp.StatusCode == http.StatusNotModified {
		log.Debugf("config not modified, using cached version")
		return c.getLatestCached(cacheDir)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response with size limit
	limitReader := io.LimitReader(resp.Body, c.maxResponseSize)
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON
	var config AutoConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate config
	if config.AutoConfigVersion == 0 {
		return nil, fmt.Errorf("invalid config: missing AutoConfigVersion")
	}

	// Save to cache
	if err := c.saveToCache(cacheDir, config.AutoConfigVersion, body); err != nil {
		log.Warnf("failed to save to cache: %v", err)
	}

	// Save metadata
	newEtag := resp.Header.Get("ETag")
	newLastModified := resp.Header.Get("Last-Modified")
	if err := c.writeCachedMetadata(cacheDir, newEtag, newLastModified); err != nil {
		log.Warnf("failed to save metadata: %v", err)
	}

	log.Infof("fetched autoconfig version %d", config.AutoConfigVersion)
	return &config, nil
}

// getLatestCached returns the latest cached config
func (c *Client) getLatestCached(cacheDir string) (*AutoConfig, error) {
	versions, err := c.getCachedVersions(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached versions: %w", err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no cached versions available")
	}

	// Get the latest version (versions are sorted in descending order)
	latestVersion := versions[0]
	filename := filepath.Join(cacheDir, fmt.Sprintf("%d.json", latestVersion))

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read cached config: %w", err)
	}

	var config AutoConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cached config: %w", err)
	}

	return &config, nil
}

// getCachedVersions returns all cached versions sorted in descending order
func (c *Client) getCachedVersions(cacheDir string) ([]int64, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache dir: %w", err)
	}

	var versions []int64
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			name := strings.TrimSuffix(entry.Name(), ".json")
			if version, err := strconv.ParseInt(name, 10, 64); err == nil {
				versions = append(versions, version)
			}
		}
	}

	// Sort in descending order (latest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] > versions[j]
	})

	return versions, nil
}

// saveToCache saves config to cache
func (c *Client) saveToCache(cacheDir string, version int64, data []byte) error {
	filename := filepath.Join(cacheDir, fmt.Sprintf("%d.json", version))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	return nil
}

// cleanupOldVersions removes old cached versions beyond maxVersions
func (c *Client) cleanupOldVersions(cacheDir string) error {
	versions, err := c.getCachedVersions(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to get cached versions: %w", err)
	}

	if len(versions) <= c.cacheSize {
		return nil
	}

	// Remove versions beyond cacheSize
	for _, version := range versions[c.cacheSize:] {
		filename := filepath.Join(cacheDir, fmt.Sprintf("%d.json", version))
		if err := os.Remove(filename); err != nil {
			log.Warnf("failed to remove old cache file %s: %v", filename, err)
		} else {
			log.Debugf("removed old cache file: %s", filename)
		}
	}

	return nil
}
