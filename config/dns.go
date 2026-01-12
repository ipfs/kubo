package config

// DNS specifies DNS resolution rules using custom resolvers.
type DNS struct {
	// Resolvers is a map of FQDNs to URLs for custom DNS resolution.
	// URLs starting with `https://` indicate DoH endpoints.
	// Support for other resolver types can be added in the future.
	// https://en.wikipedia.org/wiki/Fully_qualified_domain_name
	// https://en.wikipedia.org/wiki/DNS_over_HTTPS
	//
	// Example:
	// - Custom resolver for ENS:          `eth.` → `https://dns.eth.limo/dns-query`
	// - Override the default OS resolver: `.`    → `https://1.1.1.1/dns-query`
	Resolvers map[string]string
	// MaxCacheTTL is the maximum duration DNS entries are valid in the cache.
	MaxCacheTTL *OptionalDuration `json:",omitempty"`
	// OverrideSystem controls whether DNS.Resolvers config is applied globally
	// to all DNS lookups performed by the daemon, including third-party libraries.
	// When enabled (default), net.DefaultResolver is replaced with one that uses
	// the configured resolvers, ensuring consistent DNS behavior across the daemon.
	// Set to false to use the OS resolver for code that doesn't explicitly use
	// the Kubo DNS resolver (useful for testing or debugging).
	OverrideSystem Flag `json:",omitempty"`
}
