package config

import "time"

const (
	DefaultReproviderInterval = time.Hour * 22 // https://github.com/ipfs/kubo/pull/9326
	DefaultReproviderStrategy = "all"

	DefaultReproviderSweepEnabled                  = false
	DefaultReproviderSweepMaxWorkers               = 4
	DefaultReproviderSweepDedicatedPeriodicWorkers = 2
	DefaultReproviderSweepDedicatedBurstWorkers    = 1
	DefaultReproviderSweepMaxProvideConnsPerWorker = 16
	DefaultReproviderSweepMHStoreBatchSize         = 1 << 14 // ~544 KiB per batch (1 multihash = 34 bytes)
)

// Reprovider configuration describes how CID from local datastore are periodically re-announced to routing systems.
// For provide behavior of ad-hoc or newly created CIDs and their first-time announcement, see Provider.*
type Reprovider struct {
	Interval *OptionalDuration `json:",omitempty"` // Time period to reprovide locally stored objects to the network
	Strategy *OptionalString   `json:",omitempty"` // Which keys to announce

	Sweep Sweep
}

// Sweep configuration describes how the Sweeping Reprovider is configured if enabled.
type Sweep struct {
	Enabled Flag `json:",omitempty"`

	MaxWorkers               *OptionalInteger // Max number of concurrent workers performing a provide operation.
	DedicatedPeriodicWorkers *OptionalInteger // Number of workers dedicated to periodic reprovides.
	DedicatedBurstWorkers    *OptionalInteger // Number of workers dedicated to initial provides or burst reproviding keyspace regions after a period of inactivity.
	MaxProvideConnsPerWorker *OptionalInteger // Number of connections that a worker is able to open to send provider records during a (re)provide operation.

	MHStoreGCInterval *OptionalDuration // Interval for garbage collection in MHStore.
	MHStoreBatchSize  *OptionalInteger  // Number of multihashes to keep in memory when gc'ing the MHStore.
}
