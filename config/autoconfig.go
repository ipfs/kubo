package config

import (
	"context"
	"math/rand"
	"path/filepath"
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
)

// DNSResolversWithAutoConfig returns DNS resolvers with "auto" values replaced by autoconfig values
func (c *Config) DNSResolversWithAutoConfig(repoPath string) map[string]string {
	if c.DNS.Resolvers == nil {
		return nil
	}

	resolved := make(map[string]string)
	autoConfig := c.getAutoConfig(repoPath)
	autoExpanded := 0

	// Process each resolver
	for domain, resolver := range c.DNS.Resolvers {
		if resolver == AutoPlaceholder {
			// Try to resolve from autoconfig
			if autoConfig != nil && autoConfig.DNSResolvers != nil {
				if resolvers, exists := autoConfig.DNSResolvers[domain]; exists && len(resolvers) > 0 {
					// Use random resolver from autoconfig for load balancing
					selectedResolver := resolvers[rand.Intn(len(resolvers))]
					resolved[domain] = selectedResolver
					autoExpanded++
				}
			}
			// If autoConfig is nil (disabled), skip this "auto" resolver - don't expand it
		} else {
			// Keep custom resolver as-is
			resolved[domain] = resolver
		}
	}

	// Add default resolvers from autoconfig that aren't already configured
	// This handles the case where "." → "auto" means "include all autoconfig resolvers"
	if autoConfig != nil && autoConfig.DNSResolvers != nil {
		for domain, resolvers := range autoConfig.DNSResolvers {
			if _, exists := resolved[domain]; !exists && len(resolvers) > 0 {
				resolved[domain] = resolvers[rand.Intn(len(resolvers))]
			}
		}
	}
	// If autoConfig is nil (disabled), don't add any default resolvers

	// Log expansion if any "auto" values were found
	if autoExpanded > 0 {
		log.Debugf("expanding 'auto' DNS.Resolvers placeholder to %d resolvers from autoconfig", autoExpanded)
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
		autoConfigData = autoConfig.Bootstrap
	}
	return expandAutoConfigSlice(c.Bootstrap, autoConfigData, "Bootstrap")
}

// getAutoConfig is a helper to get autoconfig data with fallbacks
func (c *Config) getAutoConfig(repoPath string) *autoconfig.Config {
	if !c.AutoConfig.Enabled.WithDefault(DefaultAutoConfigEnabled) || c.AutoConfig.URL == "" {
		return nil
	}

	// Normal operation - use kubo user agent and allow network access
	userAgent := version.GetUserAgentVersion()
	refreshInterval := c.AutoConfig.RefreshInterval.WithDefault(DefaultAutoConfigRefreshInterval)

	// Create client
	cacheDir := filepath.Join(repoPath, "autoconfig")
	client, err := autoconfig.NewClient(
		autoconfig.WithCacheDir(cacheDir),
		autoconfig.WithUserAgent(userAgent),
		autoconfig.WithCacheSize(3),
		autoconfig.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil
	}

	ctx := context.Background()
	return client.MustGetConfig(ctx, c.AutoConfig.URL, refreshInterval)
}

// BootstrapPeersWithAutoConfig returns bootstrap peers with "auto" values replaced by autoconfig values
// and parsed into peer.AddrInfo structures
func (c *Config) BootstrapPeersWithAutoConfig(repoPath string) ([]peer.AddrInfo, error) {
	bootstrapStrings := c.BootstrapWithAutoConfig(repoPath)
	return ParseBootstrapPeers(bootstrapStrings)
}

// DelegatedRoutersWithAutoConfig returns delegated routers with "auto" values replaced by autoconfig values
func (c *Config) DelegatedRoutersWithAutoConfig(repoPath string) []string {
	autoConfig := c.getAutoConfig(repoPath)
	nodeType := c.getNodeType()
	var autoConfigData []string
	if autoConfig != nil && autoConfig.DelegatedRouters != nil {
		if routers, exists := autoConfig.DelegatedRouters[nodeType]; exists {
			autoConfigData = routers
		}
	}
	return expandAutoConfigSlice(c.Routing.DelegatedRouters, autoConfigData, "DelegatedRouters")
}

// DelegatedPublishersWithAutoConfig returns delegated publishers with "auto" values replaced by autoconfig values
func (c *Config) DelegatedPublishersWithAutoConfig(repoPath string) []string {
	autoConfig := c.getAutoConfig(repoPath)
	var autoConfigData []string
	if autoConfig != nil && autoConfig.DelegatedPublishers != nil {
		if publishers, exists := autoConfig.DelegatedPublishers[autoconfig.MainnetProfileIPNSPublishers]; exists {
			autoConfigData = publishers
		}
	}
	return expandAutoConfigSlice(c.Ipns.DelegatedPublishers, autoConfigData, "DelegatedPublishers")
}

// ExpandAutoConfigValues expands "auto" placeholders in config with their actual values using the same methods as the daemon
func (c *Config) ExpandAutoConfigValues(cfgRoot string, cfg map[string]interface{}) (map[string]interface{}, error) {
	// Create a deep copy of the config map to avoid modifying the original
	expandedCfg := make(map[string]interface{})
	for k, v := range cfg {
		expandedCfg[k] = v
	}

	// Use the same expansion methods that the daemon uses - always expand all auto-compatible fields
	// This ensures consistency with what the daemon actually uses at runtime

	// Expand Bootstrap using the shared method
	if _, exists := expandedCfg["Bootstrap"]; exists {
		expanded := c.BootstrapWithAutoConfig(cfgRoot)
		expandedCfg["Bootstrap"] = stringSliceToInterfaceSlice(expanded)
	}

	// Expand DNS.Resolvers using the shared method
	if dns, exists := expandedCfg["DNS"]; exists {
		if dnsMap, ok := dns.(map[string]interface{}); ok {
			if _, exists := dnsMap["Resolvers"]; exists {
				expanded := c.DNSResolversWithAutoConfig(cfgRoot)
				dnsMap["Resolvers"] = stringMapToInterfaceMap(expanded)
				expandedCfg["DNS"] = dnsMap
			}
		}
	}

	// Expand Routing.DelegatedRouters using the shared method
	if routing, exists := expandedCfg["Routing"]; exists {
		if routingMap, ok := routing.(map[string]interface{}); ok {
			if _, exists := routingMap["DelegatedRouters"]; exists {
				expanded := c.DelegatedRoutersWithAutoConfig(cfgRoot)
				routingMap["DelegatedRouters"] = stringSliceToInterfaceSlice(expanded)
				expandedCfg["Routing"] = routingMap
			}
		}
	}

	// Expand Ipns.DelegatedPublishers using the shared method
	if ipns, exists := expandedCfg["Ipns"]; exists {
		if ipnsMap, ok := ipns.(map[string]interface{}); ok {
			if _, exists := ipnsMap["DelegatedPublishers"]; exists {
				expanded := c.DelegatedPublishersWithAutoConfig(cfgRoot)
				ipnsMap["DelegatedPublishers"] = stringSliceToInterfaceSlice(expanded)
				expandedCfg["Ipns"] = ipnsMap
			}
		}
	}

	return expandedCfg, nil
}

// ExpandConfigField expands auto values for a specific config field using the same methods as the daemon
func (c *Config) ExpandConfigField(key string, value interface{}, cfgRoot string) interface{} {
	switch key {
	case "Bootstrap":
		// Use the shared method from config/autoconfig.go
		expanded := c.BootstrapWithAutoConfig(cfgRoot)
		return stringSliceToInterfaceSlice(expanded)

	case "DNS.Resolvers":
		// Use the shared method from config/autoconfig.go
		expanded := c.DNSResolversWithAutoConfig(cfgRoot)
		return stringMapToInterfaceMap(expanded)

	case "Routing.DelegatedRouters":
		// Use the shared method from config/autoconfig.go
		expanded := c.DelegatedRoutersWithAutoConfig(cfgRoot)
		return stringSliceToInterfaceSlice(expanded)

	case "Ipns.DelegatedPublishers":
		// Use the shared method from config/autoconfig.go
		expanded := c.DelegatedPublishersWithAutoConfig(cfgRoot)
		return stringSliceToInterfaceSlice(expanded)
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

// getNodeType determines the node type based on Routing.Type configuration
func (c *Config) getNodeType() string {
	routingType := c.Routing.Type.WithDefault("auto")

	// Node has DHT if Routing.Type is auto, autoclient, dht, or dhtclient
	switch routingType {
	case "auto", "autoclient", "dht", "dhtclient":
		return autoconfig.MainnetProfileNodesWithDHT
	default:
		return autoconfig.MainnetProfileNodesWithoutDHT
	}
}
