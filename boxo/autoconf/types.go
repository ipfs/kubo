package autoconf

import "time"

// System constants for routing behavior classification
const (
	SystemAminoDHT = "AminoDHT"
	SystemIPNI     = "IPNI"
)

// Config represents the full autoconf.json structure
type Config struct {
	AutoConfVersion int64 `json:"AutoConfVersion"`
	AutoConfSchema  int   `json:"AutoConfSchema"`
	// AutoConfTTL specifies the server-recommended cache TTL in seconds.
	// The effective refresh interval will be the minimum of this value and the user-configured RefreshInterval.
	// If AutoConfTTL is 0 or negative, only the user RefreshInterval is used.
	// This allows servers to specify shorter cache periods during network transitions or configuration updates.
	AutoConfTTL        int                       `json:"AutoConfTTL"`
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
	Version   string // AutoConfVersion as string, or ETag, or Last-Modified
	FromCache bool
	CacheAge  time.Duration // only set when FromCache is true
}

// GetBootstrapPeers returns deduplicated bootstrap peers from the specified native systems
func (c *Config) GetBootstrapPeers(nativeSystems []string) []string {
	bootstrapSet := make(map[string]bool) // For deduplication
	var result []string

	for _, system := range nativeSystems {
		if systemConf, exists := c.SystemRegistry[system]; exists {
			if systemConf.NativeConfig != nil {
				for _, peer := range systemConf.NativeConfig.Bootstrap {
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

	for url, conf := range c.DelegatedEndpoints {
		hasIgnoredOverlap := false
		for _, system := range conf.Systems {
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
			filtered[url] = conf
		}
	}

	return filtered
}

// GetDNSResolvers returns the DNS resolvers map
func (c *Config) GetDNSResolvers() map[string][]string {
	return c.DNSResolvers
}
