package config

// DNSConfig specifies custom resolvers using DoH
type DNSConfig struct {
	// DefaultResolver, if present, is a URL for the default DoH resolver.
	// If empty, DNS resolution will use the system resolver.
	DefaultResolver string `json:",omitempty"`
	// CustomResolvers is a map of domains to URLs for custom DoH resolution.
	CustomResolvers map[string]string `json:",omitempty"`
}
