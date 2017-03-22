package config

type SwarmConfig struct {
	AddrFilters             []string
	DisableBandwidthMetrics bool
	NatPortMap              bool // default: true
}
