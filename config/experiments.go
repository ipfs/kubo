package config

type Experiments struct {
	FilestoreEnabled     bool
	UrlstoreEnabled      bool
	ShardingEnabled      bool
	GraphsyncEnabled     bool
	Libp2pStreamMounting bool
	P2pHttpProxy         bool
	StrategicProviding   bool

	// OverrideSecurityTransports overrides the set of available security
	// transports when non-empty. This option should eventually migrate some
	// place more stable.
	//
	// Default: ["tls", "secio", "noise"].
	OverrideSecurityTransports []string `json:",omitempty"`
}
