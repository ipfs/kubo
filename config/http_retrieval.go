package config

// HTTPRetrieval is the configuration object for HTTP Retrieval settings.
// Implicit defaults can be found in core/node/bitswap.go
type HTTPRetrieval struct {
	Enabled               Flag             `json:",omitempty"`
	Allowlist             []string         `json:",omitempty"`
	Denylist              []string         `json:",omitempty"`
	NumWorkers            *OptionalInteger `json:",omitempty"`
	MaxBlockSize          *OptionalString  `json:",omitempty"`
	TLSInsecureSkipVerify Flag             `json:",omitempty"`
}

const (
	DefaultHTTPRetrievalEnabled               = false // opt-in for now, until we figure out https://github.com/ipfs/specs/issues/496
	DefaultHTTPRetrievalNumWorkers            = 16
	DefaultHTTPRetrievalTLSInsecureSkipVerify = false  // only for testing with self-signed HTTPS certs
	DefaultHTTPRetrievalMaxBlockSize          = "2MiB" // matching bitswap: https://specs.ipfs.tech/bitswap-protocol/#block-sizes
)
