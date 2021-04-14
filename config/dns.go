package config

// DNS specifies DNS resolution rules using custom resolvers
type DNS struct {
	// Resolvers is a map of FQDNs to URLs for custom DNS resolution.
	// URLs starting with `https://` indicate DoH endpoints.
	// Support for other resolver types can be added in the future.
	// https://en.wikipedia.org/wiki/Fully_qualified_domain_name
	// https://en.wikipedia.org/wiki/DNS_over_HTTPS
	//
	// Example:
	// - Custom resolver for ENS:          `eth.` → `https://eth.link/dns-query`
	// - Override the default OS resolver: `.`    → `https://doh.applied-privacy.net/query`
	Resolvers map[string]string
}
