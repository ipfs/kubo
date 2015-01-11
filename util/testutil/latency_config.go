package testutil

import "time"

type LatencyConfig struct {
	BlockstoreLatency time.Duration
	NetworkLatency    time.Duration
	RoutingLatency    time.Duration
}

func (c LatencyConfig) All_Instantaneous() LatencyConfig {
	// Could use a zero value but whatever. Consistency of interface
	c.NetworkLatency = 0
	c.RoutingLatency = 0
	c.BlockstoreLatency = 0
	return c
}

func (c LatencyConfig) Network_NYtoSF() LatencyConfig {
	c.NetworkLatency = 20 * time.Millisecond
	return c
}

func (c LatencyConfig) Network_IntraDatacenter2014() LatencyConfig {
	c.NetworkLatency = 250 * time.Microsecond
	return c
}

func (c LatencyConfig) Blockstore_FastSSD2014() LatencyConfig {
	const iops = 100000
	c.BlockstoreLatency = (1 / iops) * time.Second
	return c
}

func (c LatencyConfig) Blockstore_SlowSSD2014() LatencyConfig {
	c.BlockstoreLatency = 150 * time.Microsecond
	return c
}

func (c LatencyConfig) Blockstore_7200RPM() LatencyConfig {
	c.BlockstoreLatency = 8 * time.Millisecond
	return c
}

func (c LatencyConfig) Routing_Slow() LatencyConfig {
	c.RoutingLatency = 200 * time.Millisecond
	return c
}
