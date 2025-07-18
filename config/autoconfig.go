package config

import "time"

// AutoConfig contains the configuration for the autoconfig subsystem
type AutoConfig struct {
	// URL is the HTTP(S) URL to fetch the autoconfig.json from
	// Default: "https://config.ipfs-mainnet.org/autoconfig.json"
	URL string

	// Enabled determines whether to use autoconfig
	// Default: true
	Enabled Flag `json:",omitempty"`

	// LastUpdate is the timestamp of when the autoconfig was last successfully updated
	LastUpdate *time.Time `json:",omitempty"`

	// LastVersion stores the ETag or Last-Modified value from the last successful fetch
	LastVersion string `json:",omitempty"`

	// Interval is how often to check for updates
	// Default: 24h
	Interval *OptionalDuration `json:",omitempty"`

	// TLSInsecureSkipVerify allows skipping TLS verification (for testing only)
	// Default: false
	TLSInsecureSkipVerify Flag `json:",omitempty"`
}

const (
	// DefaultAutoConfigURL is the default URL for fetching autoconfig
	DefaultAutoConfigURL = "https://config.ipfs-mainnet.org/autoconfig.json"

	// DefaultAutoConfigInterval is the default interval for checking updates
	DefaultAutoConfigInterval = 24 * time.Hour
)
