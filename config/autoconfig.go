package config

import (
	"context"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/boxo/autoconfig"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("config")

// AutoConfig contains the configuration for the autoconfig subsystem
type AutoConfig struct {
	// URL is the HTTP(S) URL to fetch the autoconfig.json from
	// Default: see boxo/autoconfig.MainnetAutoConfigURL
	URL string

	// Enabled determines whether to use autoconfig
	// Default: true
	Enabled Flag `json:",omitempty"`

	// LastUpdate is the timestamp of when the autoconfig was last successfully updated
	LastUpdate *time.Time `json:",omitempty"`

	// RefreshInterval is how often to refresh autoconfig data
	// Default: 24h
	RefreshInterval *OptionalDuration `json:",omitempty"`

	// TLSInsecureSkipVerify allows skipping TLS verification (for testing only)
	// Default: false
	TLSInsecureSkipVerify Flag `json:",omitempty"`
}

const (
	// AutoPlaceholder is the string used as a placeholder for autoconfig values
	AutoPlaceholder = "auto"

	// DefaultAutoConfigEnabled is the default value for AutoConfig.Enabled
	DefaultAutoConfigEnabled = true

	// DefaultAutoConfigURL is the default URL for fetching autoconfig
	DefaultAutoConfigURL = autoconfig.MainnetAutoConfigURL

	// DefaultAutoConfigRefreshInterval is the default interval for refreshing autoconfig data
	DefaultAutoConfigRefreshInterval = autoconfig.DefaultRefreshInterval

	// AutoConfig client configuration constants
	DefaultAutoconfigCacheSize = 3
	DefaultAutoconfigTimeout   = 5 * time.Second

	// Routing path constants
	IPNSWritePath = "/routing/v1/ipns"
)

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

// getDelegatedEndpointsForConfig is a helper that gets autoconfig and filtered endpoints
// This eliminates duplication between DelegatedRoutersWithAutoConfig and DelegatedPublishersWithAutoConfig
func (c *Config) getDelegatedEndpointsForConfig(repoPath string) (*autoconfig.Config, map[string]autoconfig.EndpointConfig) {
	autoConfig := c.getAutoConfig(repoPath)
	if autoConfig == nil {
		return nil, nil
	}

	routingType := c.Routing.Type.WithDefault(DefaultRoutingType)
	nativeSystems := GetNativeSystems(routingType)
	endpoints := autoConfig.GetDelegatedEndpoints(nativeSystems)

	return autoConfig, endpoints
}

// GetNativeSystems returns the list of systems that should be used natively based on routing type
func GetNativeSystems(routingType string) []string {
	switch routingType {
	case "dht", "dhtclient", "dhtserver":
		return []string{autoconfig.SystemAminoDHT} // Only native DHT
	case "auto", "autoclient":
		return []string{autoconfig.SystemAminoDHT} // Native DHT, delegated others
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

// DNSResolversWithAutoConfig returns DNS resolvers with "auto" values replaced by autoconfig values
func (c *Config) DNSResolversWithAutoConfig(repoPath string) map[string]string {
	if c.DNS.Resolvers == nil {
		return nil
	}

	resolved := make(map[string]string)
	autoConfig := c.getAutoConfig(repoPath)
	autoExpanded := 0

	// Process each configured resolver
	for domain, resolver := range c.DNS.Resolvers {
		if resolver == AutoPlaceholder {
			// Try to resolve from autoconfig
			if autoConfig != nil && autoConfig.DNSResolvers != nil {
				if resolvers, exists := autoConfig.DNSResolvers[domain]; exists && len(resolvers) > 0 {
					resolved[domain] = selectRandomResolver(resolvers)
					autoExpanded++
				}
			}
			// If autoConfig is disabled or domain not found, skip this "auto" resolver
		} else {
			// Keep custom resolver as-is
			resolved[domain] = resolver
		}
	}

	// Add default resolvers from autoconfig that aren't already configured
	if autoConfig != nil && autoConfig.DNSResolvers != nil {
		for domain, resolvers := range autoConfig.DNSResolvers {
			if _, exists := resolved[domain]; !exists && len(resolvers) > 0 {
				resolved[domain] = selectRandomResolver(resolvers)
			}
		}
	}

	// Log expansion statistics
	if autoExpanded > 0 {
		log.Debugf("expanded %d 'auto' DNS.Resolvers from autoconfig", autoExpanded)
	}

	return resolved
}

// expandAutoConfigSlice is a generic helper for expanding "auto" placeholders in string slices
// It handles the common pattern of: iterate through slice, expand "auto" once, keep custom values
func expandAutoConfigSlice(sourceSlice []string, autoConfigData []string, fieldName string) []string {
	var resolved []string
	autoExpanded := false

	for _, item := range sourceSlice {
		if item == AutoPlaceholder {
			// Replace with autoconfig data (only once)
			if autoConfigData != nil && !autoExpanded {
				log.Debugf("expanding 'auto' %s placeholder to %d items from autoconfig", fieldName, len(autoConfigData))
				resolved = append(resolved, autoConfigData...)
				autoExpanded = true
			}
			// If autoConfigData is nil or already expanded, skip redundant "auto" entries silently
		} else {
			// Keep custom item
			resolved = append(resolved, item)
		}
	}

	return resolved
}

// BootstrapWithAutoConfig returns bootstrap config with "auto" values replaced by autoconfig values
func (c *Config) BootstrapWithAutoConfig(repoPath string) []string {
	autoConfig := c.getAutoConfig(repoPath)
	var autoConfigData []string

	if autoConfig != nil {
		routingType := c.Routing.Type.WithDefault(DefaultRoutingType)
		nativeSystems := GetNativeSystems(routingType)
		autoConfigData = autoConfig.GetBootstrapPeers(nativeSystems)
		log.Debugf("BootstrapWithAutoConfig: routingType=%s nativeSystems=%v peers=%d",
			routingType, nativeSystems, len(autoConfigData))
	} else {
		log.Debugf("BootstrapWithAutoConfig: autoConfig disabled, using original config")
	}

	result := expandAutoConfigSlice(c.Bootstrap, autoConfigData, "Bootstrap")
	log.Debugf("BootstrapWithAutoConfig: final result contains %d peers", len(result))
	return result
}

// createAutoConfigClient creates a new autoconfig client with standard settings
func createAutoConfigClient(repoPath string) (*autoconfig.Client, error) {
	cacheDir := filepath.Join(repoPath, "autoconfig")
	userAgent := version.GetUserAgentVersion()

	return autoconfig.NewClient(
		autoconfig.WithCacheDir(cacheDir),
		autoconfig.WithUserAgent(userAgent),
		autoconfig.WithCacheSize(DefaultAutoconfigCacheSize),
		autoconfig.WithTimeout(DefaultAutoconfigTimeout),
	)
}

// getAutoConfig is a helper to get autoconfig data with fallbacks
func (c *Config) getAutoConfig(repoPath string) *autoconfig.Config {
	if !c.AutoConfig.Enabled.WithDefault(DefaultAutoConfigEnabled) {
		log.Debugf("getAutoConfig: AutoConfig disabled, returning nil")
		return nil
	}

	if c.AutoConfig.URL == "" {
		log.Debugf("getAutoConfig: AutoConfig.URL is empty, returning nil")
		return nil
	}

	// Create client with standard settings
	client, err := createAutoConfigClient(repoPath)
	if err != nil {
		log.Debugf("getAutoConfig: client creation failed - %v", err)
		return nil
	}

	// Fetch config with appropriate refresh interval
	// Use context.Background() as this is called during config operations that don't have request context
	ctx := context.Background()
	refreshInterval := c.AutoConfig.RefreshInterval.WithDefault(DefaultAutoConfigRefreshInterval)

	// MustGetConfig handles errors internally and returns nil on failure
	result := client.MustGetConfig(ctx, c.AutoConfig.URL, refreshInterval)
	if result == nil {
		log.Debugf("getAutoConfig: MustGetConfig returned nil (fetch failed)")
		return nil
	}

	log.Debugf("getAutoConfig: returning config with %d DelegatedEndpoints", len(result.DelegatedEndpoints))
	return result
}

// BootstrapPeersWithAutoConfig returns bootstrap peers with "auto" values replaced by autoconfig values
// and parsed into peer.AddrInfo structures
func (c *Config) BootstrapPeersWithAutoConfig(repoPath string) ([]peer.AddrInfo, error) {
	bootstrapStrings := c.BootstrapWithAutoConfig(repoPath)
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

// DelegatedRoutersWithAutoConfig returns delegated router URLs without trailing slashes
func (c *Config) DelegatedRoutersWithAutoConfig(repoPath string) []string {
	_, endpoints := c.getDelegatedEndpointsForConfig(repoPath)

	if endpoints == nil {
		return expandAutoConfigSlice(c.Routing.DelegatedRouters, nil, "DelegatedRouters")
	}

	var routers []string
	for baseURL, config := range endpoints {
		// Build URLs for all supported Read paths
		urls := buildEndpointURLs(baseURL, config.Read)
		routers = append(routers, urls...)
	}

	resolved := expandAutoConfigSlice(c.Routing.DelegatedRouters, routers, "DelegatedRouters")

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

// DelegatedPublishersWithAutoConfig returns delegated publisher URLs without trailing slashes
func (c *Config) DelegatedPublishersWithAutoConfig(repoPath string) []string {
	_, endpoints := c.getDelegatedEndpointsForConfig(repoPath)

	if endpoints == nil {
		return expandAutoConfigSlice(c.Ipns.DelegatedPublishers, nil, "DelegatedPublishers")
	}

	var publishers []string
	for baseURL, config := range endpoints {
		// Check if this endpoint supports IPNS write operations
		if containsPath(config.Write, IPNSWritePath) {
			fullURL := buildEndpointURL(baseURL, IPNSWritePath)
			publishers = append(publishers, fullURL)
		}
	}

	resolved := expandAutoConfigSlice(c.Ipns.DelegatedPublishers, publishers, "DelegatedPublishers")

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

// expandConfigField expands a specific config field with autoconfig values
// Handles both top-level fields ("Bootstrap") and nested fields ("DNS.Resolvers")
func (c *Config) expandConfigField(expandedCfg map[string]interface{}, fieldPath string, cfgRoot string) {
	// Check if this field supports autoconfig expansion
	expandFunc, supported := supportedAutoConfigFields[fieldPath]
	if !supported {
		return
	}

	// Handle top-level fields (no dot in path)
	if !strings.Contains(fieldPath, ".") {
		if _, exists := expandedCfg[fieldPath]; exists {
			expandedCfg[fieldPath] = expandFunc(c, cfgRoot)
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
				sectionMap[fieldName] = expandFunc(c, cfgRoot)
				expandedCfg[sectionName] = sectionMap
			}
		}
	}
}

// ExpandAutoConfigValues expands "auto" placeholders in config with their actual values using the same methods as the daemon
func (c *Config) ExpandAutoConfigValues(cfgRoot string, cfg map[string]interface{}) (map[string]interface{}, error) {
	// Create a deep copy of the config map to avoid modifying the original
	expandedCfg := copyConfigMap(cfg)

	// Use the same expansion methods that the daemon uses - ensures runtime consistency
	// Unified expansion for all supported autoconfig fields
	c.expandConfigField(expandedCfg, "Bootstrap", cfgRoot)
	c.expandConfigField(expandedCfg, "DNS.Resolvers", cfgRoot)
	c.expandConfigField(expandedCfg, "Routing.DelegatedRouters", cfgRoot)
	c.expandConfigField(expandedCfg, "Ipns.DelegatedPublishers", cfgRoot)

	return expandedCfg, nil
}

// supportedAutoConfigFields maps field keys to their expansion functions
var supportedAutoConfigFields = map[string]func(*Config, string) interface{}{
	"Bootstrap": func(c *Config, cfgRoot string) interface{} {
		expanded := c.BootstrapWithAutoConfig(cfgRoot)
		return stringSliceToInterfaceSlice(expanded)
	},
	"DNS.Resolvers": func(c *Config, cfgRoot string) interface{} {
		expanded := c.DNSResolversWithAutoConfig(cfgRoot)
		return stringMapToInterfaceMap(expanded)
	},
	"Routing.DelegatedRouters": func(c *Config, cfgRoot string) interface{} {
		expanded := c.DelegatedRoutersWithAutoConfig(cfgRoot)
		return stringSliceToInterfaceSlice(expanded)
	},
	"Ipns.DelegatedPublishers": func(c *Config, cfgRoot string) interface{} {
		expanded := c.DelegatedPublishersWithAutoConfig(cfgRoot)
		return stringSliceToInterfaceSlice(expanded)
	},
}

// ExpandConfigField expands auto values for a specific config field using the same methods as the daemon
func (c *Config) ExpandConfigField(key string, value interface{}, cfgRoot string) interface{} {
	if expandFunc, supported := supportedAutoConfigFields[key]; supported {
		return expandFunc(c, cfgRoot)
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
