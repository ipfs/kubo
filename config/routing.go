package config

// Routing defines configuration options for libp2p routing
type Routing struct {
	// Type sets default daemon routing mode.
	Type string

	// PrivateType sets the routing mode for private networks. If unset,
	// Type will be used.
	PrivateType string
}
