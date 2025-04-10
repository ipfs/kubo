package config

// BitswapConfig holds Bitswap configuration options
type BitswapConfig struct {
	// Enabled controls both client and server (enabled by default)
	Enabled Flag `json:",omitempty"`
	// ServerEnabled controls if the node responds to WANTs (depends on Enabled, enabled by default)
	ServerEnabled Flag `json:",omitempty"`
}
