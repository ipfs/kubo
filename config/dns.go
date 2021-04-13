package config

// DNSConfig specifies DNS resolution rules using custom resolvers
type DNSConfig struct {
	// CustomResolvers is a map of FQDNs to URLs for custom DNS resolution.
	Resolvers map[string]string
}
