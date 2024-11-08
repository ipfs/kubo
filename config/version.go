package config

const DefaultSwarmCheckPercentThreshold = 5

// Version allows controling things like custom user agent and update checks.
type Version struct {
	// Optional suffix to the AgentVersion presented by `ipfs id` and exposed
	// via libp2p identify protocol.
	AgentSuffix *OptionalString `json:",omitempty"`

	// Detect when to warn about new version when observed via libp2p identify
	SwarmCheckEnabled          Flag             `json:",omitempty"`
	SwarmCheckPercentThreshold *OptionalInteger `json:",omitempty"`
}
