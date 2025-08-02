package autoconfig

import "time"

// System constants for routing behavior classification
const (
	SystemAminoDHT = "AminoDHT"
	SystemIPNI     = "IPNI"
)

// Config represents the full autoconfig.json structure
type Config struct {
	AutoConfigVersion int64 `json:"AutoConfigVersion"`
	AutoConfigSchema  int   `json:"AutoConfigSchema"`
	// CacheTTL specifies the server-recommended cache TTL in seconds.
	// The effective refresh interval will be the minimum of this value and the user-configured RefreshInterval.
	// If CacheTTL is 0 or negative, only the user RefreshInterval is used.
	// This allows servers to specify shorter cache periods during network transitions or configuration updates.
	CacheTTL           int                       `json:"CacheTTL"`
	SystemRegistry     map[string]SystemConfig   `json:"SystemRegistry"`
	DNSResolvers       map[string][]string       `json:"DNSResolvers"`
	DelegatedEndpoints map[string]EndpointConfig `json:"DelegatedEndpoints"`
}

// SystemConfig represents configuration for a routing system
type SystemConfig struct {
	URL             string           `json:"URL"`
	Description     string           `json:"Description"`
	NativeConfig    *NativeConfig    `json:"NativeConfig,omitempty"`
	DelegatedConfig *DelegatedConfig `json:"DelegatedConfig,omitempty"`
}

// NativeConfig represents native configuration for a system
type NativeConfig struct {
	Bootstrap []string `json:"Bootstrap"`
}

// DelegatedConfig represents delegated configuration for a system
type DelegatedConfig struct {
	Read  []string `json:"Read"`
	Write []string `json:"Write"`
}

// EndpointConfig represents a delegated endpoint configuration
type EndpointConfig struct {
	Systems []string `json:"Systems"`
	Read    []string `json:"Read"`
	Write   []string `json:"Write"`
}

// Response contains the config and metadata from the fetch
type Response struct {
	Config    *Config
	FetchTime time.Time
	Version   string // AutoConfigVersion as string, or ETag, or Last-Modified
	FromCache bool
	CacheAge  time.Duration // only set when FromCache is true
}

// GetBootstrapPeers returns deduplicated bootstrap peers from the specified native systems
func (c *Config) GetBootstrapPeers(nativeSystems []string) []string {
	bootstrapSet := make(map[string]bool) // For deduplication
	var result []string

	for _, system := range nativeSystems {
		if systemConfig, exists := c.SystemRegistry[system]; exists {
			if systemConfig.NativeConfig != nil {
				for _, peer := range systemConfig.NativeConfig.Bootstrap {
					if !bootstrapSet[peer] {
						bootstrapSet[peer] = true
						result = append(result, peer)
					}
				}
			}
		}
	}

	return result
}

// GetDelegatedEndpoints returns endpoints that don't overlap with the specified native systems
func (c *Config) GetDelegatedEndpoints(ignoredNativeSystems []string) map[string]EndpointConfig {
	if c.DelegatedEndpoints == nil {
		return nil
	}

	filtered := make(map[string]EndpointConfig)

	for url, config := range c.DelegatedEndpoints {
		hasIgnoredOverlap := false
		for _, system := range config.Systems {
			for _, ignoredSystem := range ignoredNativeSystems {
				if system == ignoredSystem {
					hasIgnoredOverlap = true
					break
				}
			}
			if hasIgnoredOverlap {
				break
			}
		}
		if !hasIgnoredOverlap {
			filtered[url] = config
		}
	}

	return filtered
}

// GetDNSResolvers returns the DNS resolvers map
func (c *Config) GetDNSResolvers() map[string][]string {
	return c.DNSResolvers
}
