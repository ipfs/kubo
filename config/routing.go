package config

// Routing defines configuration options for libp2p routing
type Routing struct {
	// Type sets default daemon routing mode.
	//
	// Can be one of "dht", "dhtclient", "dhtserver", "none", or unset.
	Type string

	// PrivateType sets the routing mode for private networks. Can take the
	// same values as Type and defaults to Type if unset.
	PrivateType string
}
