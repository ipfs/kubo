package config

import (
	"strings"
	"time"
)

const (
	DefaultProvideEnabled  = true
	DefaultProvideStrategy = "all"

	// DHT provider defaults
	DefaultProvideDHTInterval                 = 22 * time.Hour // https://github.com/ipfs/kubo/pull/9326
	DefaultProvideDHTMaxWorkers               = 4              // Sweep provider default (more efficient)
	DefaultProvideDHTLegacyMaxWorkers         = 16             // Legacy burst provider default (backward compat)
	DefaultProvideDHTSweepEnabled             = false
	DefaultProvideDHTDedicatedPeriodicWorkers = 2
	DefaultProvideDHTDedicatedBurstWorkers    = 1
	DefaultProvideDHTMaxProvideConnsPerWorker = 16
	DefaultProvideDHTKeyStoreBatchSize        = 1 << 14 // ~544 KiB per batch (1 multihash = 34 bytes)
	DefaultProvideDHTOfflineDelay             = 2 * time.Hour
)

type ProvideStrategy int

const (
	ProvideStrategyAll ProvideStrategy = 1 << iota
	ProvideStrategyPinned
	ProvideStrategyRoots
	ProvideStrategyMFS
)

// Provide configures both immediate CID announcements (provide operations) for new content
// and periodic re-announcements of existing CIDs (reprovide operations).
// This section combines the functionality previously split between Provider and Reprovider.
type Provide struct {
	// Enabled controls whether both provide and reprovide systems are enabled.
	// When disabled, the node will not announce any content to the routing system.
	Enabled Flag `json:",omitempty"`

	// Strategy determines which CIDs are announced to the routing system.
	// Default: DefaultProvideStrategy
	Strategy *OptionalString `json:",omitempty"`

	// DHT configures DHT-specific provide and reprovide settings.
	DHT ProvideDHT
}

// ProvideDHT configures DHT provider settings for both immediate announcements
// and periodic reprovides.
type ProvideDHT struct {
	// Interval sets the time between rounds of reproviding local content
	// to the routing system. Set to "0" to disable content reproviding.
	// Default: DefaultProvideDHTInterval
	Interval *OptionalDuration `json:",omitempty"`

	// MaxWorkers sets the maximum number of concurrent workers for provide operations.
	// When SweepEnabled is false: controls NEW CID announcements only.
	// When SweepEnabled is true: controls total worker pool for all operations.
	// Default: DefaultProvideDHTLegacyMaxWorkers when SweepEnabled=false
	// Default: DefaultProvideDHTMaxWorkers when SweepEnabled=true
	MaxWorkers *OptionalInteger `json:",omitempty"`

	// SweepEnabled activates the sweeping reprovider system which spreads
	// reprovide operations over time. This will become the default in a future release.
	// Default: DefaultProvideDHTSweepEnabled
	SweepEnabled Flag `json:",omitempty"`

	// DedicatedPeriodicWorkers sets workers dedicated to periodic reprovides (sweep mode only).
	// Default: DefaultProvideDHTDedicatedPeriodicWorkers
	DedicatedPeriodicWorkers *OptionalInteger `json:",omitempty"`

	// DedicatedBurstWorkers sets workers dedicated to burst provides (sweep mode only).
	// Default: DefaultProvideDHTDedicatedBurstWorkers
	DedicatedBurstWorkers *OptionalInteger `json:",omitempty"`

	// MaxProvideConnsPerWorker sets concurrent connections per worker for sending provider records (sweep mode only).
	// Default: DefaultProvideDHTMaxProvideConnsPerWorker
	MaxProvideConnsPerWorker *OptionalInteger `json:",omitempty"`

	// KeyStoreBatchSize sets the batch size for keystore operations during reprovide refresh (sweep mode only).
	// Default: DefaultProvideDHTKeyStoreBatchSize
	KeyStoreBatchSize *OptionalInteger `json:",omitempty"`

	// OfflineDelay sets the delay after which the provider switches from Disconnected to Offline state (sweep mode only).
	// Default: DefaultProvideDHTOfflineDelay
	OfflineDelay *OptionalDuration `json:",omitempty"`
}

func ParseProvideStrategy(s string) ProvideStrategy {
	var strategy ProvideStrategy
	for _, part := range strings.Split(s, "+") {
		switch part {
		case "all", "flat", "": // special case, does not mix with others ("flat" is deprecated, maps to "all")
			return ProvideStrategyAll
		case "pinned":
			strategy |= ProvideStrategyPinned
		case "roots":
			strategy |= ProvideStrategyRoots
		case "mfs":
			strategy |= ProvideStrategyMFS
		}
	}
	return strategy
}
