package config

type HTTPRetrieval struct {
	Enabled    bool             `json:",omitempty"`
	NumWorkers *OptionalInteger `json:",omitempty"`
	Allowlist  []string         `json:",omitempty"`
	Denylist   []string         `json:",omitempty"`
}

type Experiments struct {
	FilestoreEnabled              bool
	UrlstoreEnabled               bool
	ShardingEnabled               bool `json:",omitempty"` // deprecated by autosharding: https://github.com/ipfs/kubo/pull/8527
	Libp2pStreamMounting          bool
	P2pHttpProxy                  bool //nolint
	StrategicProviding            bool
	OptimisticProvide             bool
	OptimisticProvideJobsPoolSize int
	GatewayOverLibp2p             bool `json:",omitempty"`

	HTTPRetrieval HTTPRetrieval

	GraphsyncEnabled     graphsyncEnabled                 `json:",omitempty"`
	AcceleratedDHTClient experimentalAcceleratedDHTClient `json:",omitempty"`
}
