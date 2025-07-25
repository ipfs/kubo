package autoconfig

import "time"

// Mainnet profile constants for node type classification
const (
	MainnetProfileNodesWithDHT    = "mainnet-for-nodes-with-dht"
	MainnetProfileNodesWithoutDHT = "mainnet-for-nodes-without-dht"
	MainnetProfileIPNSPublishers  = "mainnet-for-ipns-publishers-with-http"
)

// Config represents the full autoconfig.json structure
type Config struct {
	AutoConfigVersion   int64                               `json:"AutoConfigVersion"`
	AutoConfigSchema    int                                 `json:"AutoConfigSchema"`
	Bootstrap           []string                            `json:"Bootstrap"`
	DNSResolvers        map[string][]string                 `json:"DNSResolvers"`
	DelegatedRouters    map[string]DelegatedRouterConfig    `json:"DelegatedRouters"`
	DelegatedPublishers map[string]DelegatedPublisherConfig `json:"DelegatedPublishers"`
}

// DelegatedRouterConfig represents a delegated router configuration
type DelegatedRouterConfig []string

// DelegatedPublisherConfig represents a delegated publisher configuration
type DelegatedPublisherConfig []string

// Response contains the config and metadata from the fetch
type Response struct {
	Config    *Config
	FetchTime time.Time
	Version   string // AutoConfigVersion as string, or ETag, or Last-Modified
	FromCache bool
	CacheAge  time.Duration // only set when FromCache is true
}
