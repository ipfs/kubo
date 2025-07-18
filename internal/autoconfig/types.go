package autoconfig

// AutoConfig represents the full autoconfig.json structure
type AutoConfig struct {
	AutoConfigVersion   int64                               `json:"AutoConfigVersion"`
	AutoConfigSchema    int                                 `json:"AutoConfigSchema"`
	Bootstrap           []string                            `json:"Bootstrap"`
	DNSResolvers        map[string][]string                 `json:"DNSResolvers"`
	DelegatedRouters    map[string]DelegatedRouterConfig    `json:"DelegatedRouters"`
	DelegatedPublishers map[string]DelegatedPublisherConfig `json:"DelegatedPublishers"`
}

// DelegatedRouterConfig represents a delegated router configuration
type DelegatedRouterConfig struct {
	Providers []string `json:"providers,omitempty"`
	Peers     []string `json:"peers,omitempty"`
	IPNS      []string `json:"ipns,omitempty"`
}

// DelegatedPublisherConfig represents a delegated publisher configuration
type DelegatedPublisherConfig struct {
	IPNS []string `json:"ipns,omitempty"`
}
