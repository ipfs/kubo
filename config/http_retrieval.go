package config

// HTTPRetrieval is the configuration object for HTTP Retrieval settings.
type HTTPRetrieval struct {
	Enabled    bool             `json:",omitempty"`
	NumWorkers *OptionalInteger `json:",omitempty"`
	Allowlist  []string         `json:",omitempty"`
	Denylist   []string         `json:",omitempty"`
}
