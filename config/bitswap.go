package config

// Bitswap holds Bitswap configuration options
type Bitswap struct {
	// Libp2pEnabled controls if the node initializes bitswap over libp2p (enabled by default)
	// (This can be disabled if HTTPRetrieval.Enabled is set to true)
	Libp2pEnabled Flag `json:",omitempty"`
	// ServerEnabled controls if the node responds to WANTs (depends on Libp2pEnabled, enabled by default)
	ServerEnabled Flag `json:",omitempty"`
}

const (
	DefaultBitswapLibp2pEnabled = true
	DefaultBitswapServerEnabled = true
)
