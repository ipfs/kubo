package autoconf

import (
	"slices"
	"time"
)

// AutoPlaceholder is the string used as a placeholder for autoconf values
const AutoPlaceholder = "auto"

// System constants for routing behavior classification
const (
	SystemAminoDHT = "AminoDHT"
	SystemIPNI     = "IPNI"
)

// Schema version constants
const (
	// SupportedAutoConfSchema is the schema version this client supports
	// This matches the current mainnet autoconf.json schema version
	SupportedAutoConfSchema = 1
)

// Config represents the full autoconf.json structure that defines network configuration for IPFS nodes.
//
// The configuration defines three main components:
//   - SystemRegistry: Catalogs of known routing systems and their capabilities
//   - DelegatedEndpoints: HTTP endpoints that implement specific routing operations
//   - DNSResolvers: DNS-over-HTTPS resolvers for .eth and other domains
//
// Key relationship: SystemRegistry lists what operations each system supports (via DelegatedConfig),
// while DelegatedEndpoints maps HTTP base URLs to the actual routing paths they serve.
// A single HTTP endpoint in DelegatedEndpoints can serve multiple systems, or a system can
// use endpoints from multiple HTTP servers.
type Config struct {
	AutoConfVersion int64 `json:"AutoConfVersion"`
	AutoConfSchema  int   `json:"AutoConfSchema"`
	// AutoConfTTL specifies the server-recommended cache TTL in seconds.
	// The effective refresh interval will be the minimum of this value and the user-configured RefreshInterval.
	// If AutoConfTTL is 0 or negative, only the user RefreshInterval is used.
	// This allows servers to specify shorter cache periods during network transitions or configuration updates.
	AutoConfTTL int `json:"AutoConfTTL"`

	// SystemRegistry catalogs known routing systems and their supported operations.
	// Each system defines what delegated routing paths it supports and may include
	// native configuration like bootstrap peers for systems that run locally.
	SystemRegistry map[string]SystemConfig `json:"SystemRegistry"`

	// DNSResolvers maps DNS domains to their DoH (DNS-over-HTTPS) resolver URLs.
	// Used for resolving non-ICANN TLDs or forcing all DNSLink resolution over specific DoH endpoint (in private swarms).
	DNSResolvers map[string][]string `json:"DNSResolvers"`

	// DelegatedEndpoints maps HTTP base URLs to their supported operations and protocols.
	// Each server can support multiple endpoints, and while /routing/v1 is the most popular,
	// other protocols can be added and communicated this way, and may not be specific to routing
	// but serve other purposes. The Systems field indicates which entries from SystemRegistry use this endpoint.
	DelegatedEndpoints map[string]EndpointConfig `json:"DelegatedEndpoints"`
}

// SystemConfig represents configuration for a routing system like AminoDHT or IPNI.
// Systems can operate natively (with local bootstrap peers) and/or through delegated HTTP endpoints.
type SystemConfig struct {
	URL         string `json:"URL"`         // Documentation URL for this system
	Description string `json:"Description"` // Human-readable description

	// NativeConfig contains bootstrap peers and other settings for running this system locally.
	// Present when the system can/should run natively on the node (e.g., AminoDHT).
	NativeConfig *NativeConfig `json:"NativeConfig,omitempty"`

	// DelegatedConfig lists the HTTP routing paths this system supports when used through delegation.
	// These paths correspond to entries that should exist in DelegatedEndpoints.
	DelegatedConfig *DelegatedConfig `json:"DelegatedConfig,omitempty"`
}

// NativeConfig contains settings for running a routing system locally on the node.
type NativeConfig struct {
	Bootstrap []string `json:"Bootstrap"` // Bootstrap peer multiaddrs for connecting to this system
}

// DelegatedConfig specifies which HTTP routing operations a system supports when delegated.
// These paths define the system's capabilities and must be implemented by endpoints in DelegatedEndpoints.
type DelegatedConfig struct {
	Read  []string `json:"Read"`  // HTTP paths for read operations (e.g., ["/routing/v1/providers", "/routing/v1/peers"])
	Write []string `json:"Write"` // HTTP paths for write operations (e.g., ["/routing/v1/ipns"])
}

// EndpointConfig describes an HTTP endpoint that implements delegated operations.
// The key relationship: Systems lists which SystemRegistry entries use this endpoint,
// while Read/Write list the actual HTTP paths this endpoint serves.
//
// Read and Write endpoints can be a subset of endpoints defined in SystemRegistry,
// meaning HTTP endpoints may only support a subset of features that a system offers.
//
// Example: An endpoint at "https://delegated-ipfs.dev" might serve operations for both
// "IPNI" and "CustomIPNS" systems, implementing paths like "/routing/v1/providers" and "/routing/v1/ipns".
type EndpointConfig struct {
	Systems []string `json:"Systems"` // Names of systems from SystemRegistry that use this endpoint
	Read    []string `json:"Read"`    // HTTP paths this endpoint supports for read operations
	Write   []string `json:"Write"`   // HTTP paths this endpoint supports for write operations
}

// Response contains the config and metadata from the fetch
type Response struct {
	Config    *Config
	FetchTime time.Time
	Version   string        // AutoConfVersion as string, or ETag, or Last-Modified
	CacheAge  time.Duration // non-zero when response is from cache
}

// FromCache returns true if the response was served from cache
func (r *Response) FromCache() bool {
	return r.CacheAge > 0
}

// GetBootstrapPeers returns deduplicated bootstrap peers from the specified native systems
func (c *Config) GetBootstrapPeers(nativeSystems ...string) []string {
	bootstrapSet := make(map[string]struct{}) // For deduplication
	var result []string

	for _, system := range nativeSystems {
		if systemConf, exists := c.SystemRegistry[system]; exists {
			if systemConf.NativeConfig != nil {
				for _, peer := range systemConf.NativeConfig.Bootstrap {
					if _, exists := bootstrapSet[peer]; !exists {
						bootstrapSet[peer] = struct{}{}
						result = append(result, peer)
					}
				}
			}
		}
	}

	return result
}

// GetDelegatedEndpoints returns endpoints that don't overlap with the specified native systems
func (c *Config) GetDelegatedEndpoints(ignoredNativeSystems ...string) map[string]EndpointConfig {
	if c.DelegatedEndpoints == nil {
		return nil
	}

	filtered := make(map[string]EndpointConfig)

	for url, conf := range c.DelegatedEndpoints {
		hasIgnoredOverlap := false
		for _, system := range conf.Systems {
			if slices.Contains(ignoredNativeSystems, system) {
				hasIgnoredOverlap = true
				break
			}
		}
		if !hasIgnoredOverlap {
			filtered[url] = conf
		}
	}

	log.Debugf("GetDelegatedEndpoints: returning %d endpoints from %d total", len(filtered), len(c.DelegatedEndpoints))
	return filtered
}

// GetDNSResolvers returns the DNS resolvers map
func (c *Config) GetDNSResolvers() map[string][]string {
	return c.DNSResolvers
}
