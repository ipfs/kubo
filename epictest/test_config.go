package epictest

import "time"

type Config struct {
	BlockstoreLatency time.Duration
	NetworkLatency    time.Duration
	RoutingLatency    time.Duration
}

func (c Config) All_Instantaneous() Config {
	// Could use a zero value but whatever. Consistency of interface
	c.NetworkLatency = 0
	c.RoutingLatency = 0
	c.BlockstoreLatency = 0
	return c
}

func (c Config) Network_NYtoSF() Config {
	c.NetworkLatency = 20 * time.Millisecond
	return c
}

func (c Config) Network_IntraDatacenter2014() Config {
	c.NetworkLatency = 250 * time.Microsecond
	return c
}

func (c Config) Blockstore_FastSSD2014() Config {
	const iops = 100000
	c.BlockstoreLatency = (1 / iops) * time.Second
	return c
}

func (c Config) Blockstore_SlowSSD2014() Config {
	c.BlockstoreLatency = 150 * time.Microsecond
	return c
}

func (c Config) Blockstore_7200RPM() Config {
	c.BlockstoreLatency = 8 * time.Millisecond
	return c
}

func (c Config) Routing_Slow() Config {
	c.BlockstoreLatency = 200 * time.Millisecond
	return c
}
