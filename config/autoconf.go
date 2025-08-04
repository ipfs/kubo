package config

import (
	"fmt"
	"math/rand"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/boxo/autoconf"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("config")

// clientCache provides a simple singleton pattern for autoconf clients
var (
	clientCache   = make(map[string]*autoconf.Client)
	clientCacheMu sync.RWMutex
)

// AutoConf contains the configuration for the autoconf subsystem
type AutoConf struct {
	// URL is the HTTP(S) URL to fetch the autoconf.json from
	// Default: see boxo/autoconf.MainnetAutoConfURL
	URL string

	// Enabled determines whether to use autoconf
	// Default: true
	Enabled Flag `json:",omitempty"`

	// RefreshInterval is how often to refresh autoconf data
	// Default: 24h
	RefreshInterval *OptionalDuration `json:",omitempty"`

	// TLSInsecureSkipVerify allows skipping TLS verification (for testing only)
	// Default: false
	TLSInsecureSkipVerify Flag `json:",omitempty"`
}

const (
	// AutoPlaceholder is the string used as a placeholder for autoconf values
	AutoPlaceholder = "auto"

	// DefaultAutoConfEnabled is the default value for AutoConf.Enabled
	DefaultAutoConfEnabled = true

	// DefaultAutoConfURL is the default URL for fetching autoconf
	DefaultAutoConfURL = autoconf.MainnetAutoConfURL

	// DefaultAutoConfRefreshInterval is the default interval for refreshing autoconf data
	DefaultAutoConfRefreshInterval = autoconf.DefaultRefreshInterval

	// AutoConf client configuration constants
	DefaultAutoConfCacheSize = 3
	DefaultAutoConfTimeout   = 5 * time.Second

	// Routing path constants
	IPNSWritePath = "/routing/v1/ipns"
)

// ParseAndValidateRoutingURL extracts base URL and validates routing path in one step
// Returns error if URL is invalid or has unsupported routing path
func ParseAndValidateRoutingURL(endpoint string) (baseURL string, path string, err error) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL %q: %w", endpoint, err)
	}

	// Build base URL without path
	baseURL = fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// Extract and normalize path
	path = strings.TrimPrefix(parsedURL.Path, "/")
	path = strings.TrimSuffix(path, "/")

	// Validate routing path
	switch path {
	case "": // No path - base URL
	case "routing/v1/providers": // Provider lookups
	case "routing/v1/peers": // Peer lookups
	case "routing/v1/ipns": // IPNS resolution/publishing
		// Valid paths - continue
	default:
		return "", "", fmt.Errorf("unsupported routing path %q", path)
	}

	return baseURL, path, nil
}

// filterValidRoutingURLs filters out URLs with unsupported routing paths
func filterValidRoutingURLs(urls []string) []string {
	var filtered []string
	for _, urlStr := range urls {
		if _, _, err := ParseAndValidateRoutingURL(urlStr); err == nil {
			filtered = append(filtered, urlStr)
		} else {
			log.Debugf("Skipping invalid routing URL %q: %v", urlStr, err)
		}
	}
	return filtered
}

// buildEndpointURL constructs a URL from baseURL and path, ensuring no trailing slash
func buildEndpointURL(baseURL, path string) string {
	// Always trim trailing slash from baseURL
	cleanBase := strings.TrimRight(baseURL, "/")

	// Ensure path starts with / if not empty
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Construct and ensure no trailing slash
	fullURL := cleanBase + path
	return strings.TrimRight(fullURL, "/")
}

// getDelegatedEndpointsForConfig is a helper that gets autoconf and filtered endpoints
// This eliminates duplication between DelegatedRoutersWithAutoConf and DelegatedPublishersWithAutoConf
func (c *Config) getDelegatedEndpointsForConfig() (*autoconf.Config, map[string]autoconf.EndpointConfig) {
	autoConf := c.getAutoConf()
	if autoConf == nil {
		return nil, nil
	}

	routingType := c.Routing.Type.WithDefault(DefaultRoutingType)
	nativeSystems := GetNativeSystems(routingType)
	endpoints := autoConf.GetDelegatedEndpoints(nativeSystems)

	return autoConf, endpoints
}

// GetNativeSystems returns the list of systems that should be used natively based on routing type
func GetNativeSystems(routingType string) []string {
	switch routingType {
	case "dht", "dhtclient", "dhtserver":
		return []string{autoconf.SystemAminoDHT} // Only native DHT
	case "auto", "autoclient":
		return []string{autoconf.SystemAminoDHT} // Native DHT, delegated others
	case "delegated":
		return []string{} // Everything delegated
	case "none":
		return []string{} // No native systems
	default:
		return []string{} // Custom mode
	}
}

// selectRandomResolver picks a random resolver from a list for load balancing
func selectRandomResolver(resolvers []string) string {
	if len(resolvers) == 0 {
		return ""
	}
	return resolvers[rand.Intn(len(resolvers))]
}

// DNSResolversWithAutoConf returns DNS resolvers with "auto" values replaced by autoconf values
func (c *Config) DNSResolversWithAutoConf() map[string]string {
	if c.DNS.Resolvers == nil {
		return nil
	}

	resolved := make(map[string]string)
	autoConf := c.getAutoConf()
	autoExpanded := 0

	// Process each configured resolver
	for domain, resolver := range c.DNS.Resolvers {
		if resolver == AutoPlaceholder {
			// Try to resolve from autoconf
			if autoConf != nil && autoConf.DNSResolvers != nil {
				if resolvers, exists := autoConf.DNSResolvers[domain]; exists && len(resolvers) > 0 {
					resolved[domain] = selectRandomResolver(resolvers)
					autoExpanded++
				}
			}
			// If autoConf is disabled or domain not found, skip this "auto" resolver
		} else {
			// Keep custom resolver as-is
			resolved[domain] = resolver
		}
	}

	// Add default resolvers from autoconf that aren't already configured
	if autoConf != nil && autoConf.DNSResolvers != nil {
		for domain, resolvers := range autoConf.DNSResolvers {
			if _, exists := resolved[domain]; !exists && len(resolvers) > 0 {
				resolved[domain] = selectRandomResolver(resolvers)
			}
		}
	}

	// Log expansion statistics
	if autoExpanded > 0 {
		log.Debugf("expanded %d 'auto' DNS.Resolvers from autoconf", autoExpanded)
	}

	return resolved
}

// expandAutoConfSlice is a generic helper for expanding "auto" placeholders in string slices
// It handles the common pattern of: iterate through slice, expand "auto" once, keep custom values
func expandAutoConfSlice(sourceSlice []string, autoConfData []string, fieldName string) []string {
	var resolved []string
	autoExpanded := false

	for _, item := range sourceSlice {
		if item == AutoPlaceholder {
			// Replace with autoconf data (only once)
			if autoConfData != nil && !autoExpanded {
				log.Debugf("expanding 'auto' %s placeholder to %d items from autoconf", fieldName, len(autoConfData))
				resolved = append(resolved, autoConfData...)
				autoExpanded = true
			}
			// If autoConfData is nil or already expanded, skip redundant "auto" entries silently
		} else {
			// Keep custom item
			resolved = append(resolved, item)
		}
	}

	return resolved
}

// BootstrapWithAutoConf returns bootstrap config with "auto" values replaced by autoconf values
func (c *Config) BootstrapWithAutoConf() []string {
	autoConf := c.getAutoConf()
	var autoConfData []string

	if autoConf != nil {
		routingType := c.Routing.Type.WithDefault(DefaultRoutingType)
		nativeSystems := GetNativeSystems(routingType)
		autoConfData = autoConf.GetBootstrapPeers(nativeSystems)
		log.Debugf("BootstrapWithAutoConf: processing with routing type: %s", routingType)
	} else {
		log.Debugf("BootstrapWithAutoConf: autoConf disabled, using original config")
	}

	result := expandAutoConfSlice(c.Bootstrap, autoConfData, "Bootstrap")
	log.Debugf("BootstrapWithAutoConf: final result contains %d peers", len(result))
	return result
}

// getOrCreateAutoConfClient returns a cached client or creates a new one with standard settings
func getOrCreateAutoConfClient() (*autoconf.Client, error) {
	// Use standard config path resolution
	repoPath, err := PathRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo path: %w", err)
	}

	clientCacheMu.RLock()
	if client, exists := clientCache[repoPath]; exists {
		clientCacheMu.RUnlock()
		return client, nil
	}
	clientCacheMu.RUnlock()

	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()

	// Double-check after acquiring write lock
	if client, exists := clientCache[repoPath]; exists {
		return client, nil
	}

	cacheDir := filepath.Join(repoPath, "autoconf")
	userAgent := version.GetUserAgentVersion()

	client, err := autoconf.NewClient(
		autoconf.WithCacheDir(cacheDir),
		autoconf.WithUserAgent(userAgent),
		autoconf.WithCacheSize(DefaultAutoConfCacheSize),
		autoconf.WithTimeout(DefaultAutoConfTimeout),
	)
	if err != nil {
		return nil, err
	}

	clientCache[repoPath] = client
	return client, nil
}

// getAutoConf is a helper to get autoconf data with fallbacks
func (c *Config) getAutoConf() *autoconf.Config {
	if !c.AutoConf.Enabled.WithDefault(DefaultAutoConfEnabled) {
		log.Debugf("getAutoConf: AutoConf disabled, returning nil")
		return nil
	}

	if c.AutoConf.URL == "" {
		log.Debugf("getAutoConf: AutoConf.URL is empty, returning nil")
		return nil
	}

	// Create or get cached client with standard settings
	client, err := getOrCreateAutoConfClient()
	if err != nil {
		log.Debugf("getAutoConf: client creation failed - %v", err)
		return nil
	}

	// Use MustGetConfigCached to avoid network I/O during config operations
	// This ensures config retrieval doesn't block on network operations
	result := client.MustGetConfigCached(c.AutoConf.URL, autoconf.GetMainnetFallbackConfig)

	log.Debugf("getAutoConf: returning autoconf data")
	return result
}

// BootstrapPeersWithAutoConf returns bootstrap peers with "auto" values replaced by autoconf values
// and parsed into peer.AddrInfo structures
func (c *Config) BootstrapPeersWithAutoConf() ([]peer.AddrInfo, error) {
	bootstrapStrings := c.BootstrapWithAutoConf()
	return ParseBootstrapPeers(bootstrapStrings)
}

// buildEndpointURLs creates URLs from base URL and paths, ensuring no trailing slashes
func buildEndpointURLs(baseURL string, paths []string) []string {
	var urls []string
	for _, path := range paths {
		url := buildEndpointURL(baseURL, path)
		urls = append(urls, url)
	}
	return urls
}

// DelegatedRoutersWithAutoConf returns delegated router URLs without trailing slashes
func (c *Config) DelegatedRoutersWithAutoConf() []string {
	_, endpoints := c.getDelegatedEndpointsForConfig()

	if endpoints == nil {
		return expandAutoConfSlice(c.Routing.DelegatedRouters, nil, "DelegatedRouters")
	}

	var routers []string
	for baseURL, config := range endpoints {
		// Build URLs for all supported Read paths
		urls := buildEndpointURLs(baseURL, config.Read)
		routers = append(routers, urls...)
	}

	resolved := expandAutoConfSlice(c.Routing.DelegatedRouters, routers, "DelegatedRouters")

	// Filter out URLs with unsupported routing paths
	resolved = filterValidRoutingURLs(resolved)

	// Final safety check to guarantee no trailing slashes
	for i, url := range resolved {
		resolved[i] = strings.TrimRight(url, "/")
	}

	return resolved
}

// containsPath checks if the given paths contain the target path
func containsPath(paths []string, targetPath string) bool {
	for _, path := range paths {
		if path == targetPath {
			return true
		}
	}
	return false
}

// DelegatedPublishersWithAutoConf returns delegated publisher URLs without trailing slashes
func (c *Config) DelegatedPublishersWithAutoConf() []string {
	_, endpoints := c.getDelegatedEndpointsForConfig()

	if endpoints == nil {
		return expandAutoConfSlice(c.Ipns.DelegatedPublishers, nil, "DelegatedPublishers")
	}

	var publishers []string
	for baseURL, config := range endpoints {
		// Check if this endpoint supports IPNS write operations
		if containsPath(config.Write, IPNSWritePath) {
			fullURL := buildEndpointURL(baseURL, IPNSWritePath)
			publishers = append(publishers, fullURL)
		}
	}

	resolved := expandAutoConfSlice(c.Ipns.DelegatedPublishers, publishers, "DelegatedPublishers")

	// Filter out URLs with unsupported routing paths
	resolved = filterValidRoutingURLs(resolved)

	// Final safety check to guarantee no trailing slashes
	for i, url := range resolved {
		resolved[i] = strings.TrimRight(url, "/")
	}

	return resolved
}

// copyConfigMap creates a deep copy of a config map to avoid modifying the original
func copyConfigMap(cfg map[string]interface{}) map[string]interface{} {
	copied := make(map[string]interface{})
	for k, v := range cfg {
		copied[k] = v
	}
	return copied
}

// expandConfigField expands a specific config field with autoconf values
// Handles both top-level fields ("Bootstrap") and nested fields ("DNS.Resolvers")
func (c *Config) expandConfigField(expandedCfg map[string]interface{}, fieldPath string) {
	// Check if this field supports autoconf expansion
	expandFunc, supported := supportedAutoConfFields[fieldPath]
	if !supported {
		return
	}

	// Handle top-level fields (no dot in path)
	if !strings.Contains(fieldPath, ".") {
		if _, exists := expandedCfg[fieldPath]; exists {
			expandedCfg[fieldPath] = expandFunc(c)
		}
		return
	}

	// Handle nested fields (section.field format)
	parts := strings.SplitN(fieldPath, ".", 2)
	if len(parts) != 2 {
		return
	}

	sectionName, fieldName := parts[0], parts[1]
	if section, exists := expandedCfg[sectionName]; exists {
		if sectionMap, ok := section.(map[string]interface{}); ok {
			if _, exists := sectionMap[fieldName]; exists {
				sectionMap[fieldName] = expandFunc(c)
				expandedCfg[sectionName] = sectionMap
			}
		}
	}
}

// ExpandAutoConfValues expands "auto" placeholders in config with their actual values using the same methods as the daemon
func (c *Config) ExpandAutoConfValues(cfg map[string]interface{}) (map[string]interface{}, error) {
	// Create a deep copy of the config map to avoid modifying the original
	expandedCfg := copyConfigMap(cfg)

	// Use the same expansion methods that the daemon uses - ensures runtime consistency
	// Unified expansion for all supported autoconf fields
	c.expandConfigField(expandedCfg, "Bootstrap")
	c.expandConfigField(expandedCfg, "DNS.Resolvers")
	c.expandConfigField(expandedCfg, "Routing.DelegatedRouters")
	c.expandConfigField(expandedCfg, "Ipns.DelegatedPublishers")

	return expandedCfg, nil
}

// supportedAutoConfFields maps field keys to their expansion functions
var supportedAutoConfFields = map[string]func(*Config) interface{}{
	"Bootstrap": func(c *Config) interface{} {
		expanded := c.BootstrapWithAutoConf()
		return stringSliceToInterfaceSlice(expanded)
	},
	"DNS.Resolvers": func(c *Config) interface{} {
		expanded := c.DNSResolversWithAutoConf()
		return stringMapToInterfaceMap(expanded)
	},
	"Routing.DelegatedRouters": func(c *Config) interface{} {
		expanded := c.DelegatedRoutersWithAutoConf()
		return stringSliceToInterfaceSlice(expanded)
	},
	"Ipns.DelegatedPublishers": func(c *Config) interface{} {
		expanded := c.DelegatedPublishersWithAutoConf()
		return stringSliceToInterfaceSlice(expanded)
	},
}

// ExpandConfigField expands auto values for a specific config field using the same methods as the daemon
func (c *Config) ExpandConfigField(key string, value interface{}) interface{} {
	if expandFunc, supported := supportedAutoConfFields[key]; supported {
		return expandFunc(c)
	}

	// Return original value if no expansion needed (not a field that supports auto values)
	return value
}

// Helper functions for type conversion between string types and interface{} types for JSON compatibility

func stringSliceToInterfaceSlice(slice []string) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

func stringMapToInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}
