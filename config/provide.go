package config

import (
	"time"
)

const (
	DefaultProvideEnabled     = true
	DefaultProvideWorkerCount = 16
	DefaultProvideInterval    = time.Hour * 22 // https://github.com/ipfs/kubo/pull/9326
	DefaultProvideStrategy    = "all"

	DefaultProvideSweepEnabled                  = false
	DefaultProvideSweepMaxWorkers               = 4
	DefaultProvideSweepDedicatedPeriodicWorkers = 2
	DefaultProvideSweepDedicatedBurstWorkers    = 1
	DefaultProvideSweepMaxProvideConnsPerWorker = 16
	DefaultProvideSweepKeyStoreBatchSize        = 1 << 14 // ~544 KiB per batch (1 multihash = 34 bytes)
	DefaultProvideSweepOfflineDelay             = 2 * time.Hour
)

// Provide configures both immediate CID announcements (provide operations) for new content
// and periodic re-announcements of existing CIDs (reprovide operations).
// This section combines the functionality previously split between Provider and Reprovider.
type Provide struct {
	// Enabled controls whether both provide and reprovide systems are enabled.
	// When disabled, the node will not announce any content to the routing system.
	Enabled Flag `json:",omitempty"`

	// Strategy determines which CIDs are announced to the routing system.
	// Default: DefaultProvideStrategy ("all")
	Strategy *OptionalString `json:",omitempty"`

	// WorkerCount sets the maximum number of concurrent DHT provide operations
	// for NEW CID announcements only. Reprovide operations do not count against this limit.
	// A value of 0 allows unlimited workers. Default: DefaultProvideWorkerCount
	WorkerCount *OptionalInteger `json:",omitempty"`

	// ReprovideInterval sets the time between rounds of reproviding local content
	// to the routing system. Set to "0" to disable content reproviding.
	// Default: DefaultProvideInterval
	ReprovideInterval *OptionalDuration `json:",omitempty"`

	// Sweep configures the sweeping reprovider for efficient DHT announcements.
	// When enabled, it spreads reprovide operations over time by sweeping the keyspace.
	Sweep ProvideSweep `json:",omitempty"`
}

// ProvideSweep configures the sweeping reprovider which spreads reprovide operations
// over time by sweeping the DHT keyspace. This reduces resource spikes compared to
// reproviding all CIDs at once.
type ProvideSweep struct {
	// Enabled activates the sweeping reprovider system.
	// Default: DefaultProvideSweepEnabled
	Enabled Flag `json:",omitempty"`

	// MaxWorkers sets the maximum number of workers for provide/reprovide operations.
	// Default: DefaultProvideSweepMaxWorkers
	MaxWorkers *OptionalInteger `json:",omitempty"`

	// DedicatedPeriodicWorkers sets workers dedicated to periodic reprovides.
	// Default: DefaultProvideSweepDedicatedPeriodicWorkers
	DedicatedPeriodicWorkers *OptionalInteger `json:",omitempty"`

	// DedicatedBurstWorkers sets workers dedicated to burst provides.
	// Default: DefaultProvideSweepDedicatedBurstWorkers
	DedicatedBurstWorkers *OptionalInteger `json:",omitempty"`

	// MaxProvideConnsPerWorker sets concurrent connections per worker for sending provider records.
	// Default: DefaultProvideSweepMaxProvideConnsPerWorker
	MaxProvideConnsPerWorker *OptionalInteger `json:",omitempty"`

	// KeyStoreBatchSize sets the batch size for keystore operations during reprovide refresh.
	// Default: DefaultProvideSweepKeyStoreBatchSize
	KeyStoreBatchSize *OptionalInteger `json:",omitempty"`

	// OfflineDelay sets the delay after which the provider switches from Disconnected to Offline state.
	// Default: DefaultProvideSweepOfflineDelay
	OfflineDelay *OptionalDuration `json:",omitempty"`
}
