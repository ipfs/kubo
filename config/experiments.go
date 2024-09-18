package config

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

	GraphsyncEnabled     graphsyncEnabled                 `json:",omitempty"`
	AcceleratedDHTClient experimentalAcceleratedDHTClient `json:",omitempty"`
}
