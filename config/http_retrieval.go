package config

// HTTPRetrieval is the configuration object for HTTP Retrieval settings.
// Implicit defaults can be found in core/node/bitswap.go
type HTTPRetrieval struct {
	Enabled    Flag             `json:",omitempty"`
	NumWorkers *OptionalInteger `json:",omitempty"`
	Allowlist  []string         `json:",omitempty"`
	Denylist   []string         `json:",omitempty"`
}

const (
	DefaultHTTPRetrievalEnabled    = false // opt-in for now, until we figure out https://github.com/ipfs/specs/issues/496
	DefaultHTTPRetrievalNumWorkers = 16
)
