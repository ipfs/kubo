package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-kad-dht/amino"
)

const (
	DefaultProvideEnabled  = true
	DefaultProvideStrategy = "all"

	// DHT provider defaults
	DefaultProvideDHTInterval                 = 22 * time.Hour // https://github.com/ipfs/kubo/pull/9326
	DefaultProvideDHTMaxWorkers               = 16             // Unified default for both sweep and legacy providers
	DefaultProvideDHTSweepEnabled             = false
	DefaultProvideDHTDedicatedPeriodicWorkers = 2
	DefaultProvideDHTDedicatedBurstWorkers    = 1
	DefaultProvideDHTMaxProvideConnsPerWorker = 20
	DefaultProvideDHTKeystoreBatchSize        = 1 << 14 // ~544 KiB per batch (1 multihash = 34 bytes)
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
	// Default: DefaultProvideDHTMaxWorkers
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

	// KeystoreBatchSize sets the batch size for keystore operations during reprovide refresh (sweep mode only).
	// Default: DefaultProvideDHTKeystoreBatchSize
	KeystoreBatchSize *OptionalInteger `json:",omitempty"`

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

// ValidateProvideConfig validates the Provide configuration according to DHT requirements.
func ValidateProvideConfig(cfg *Provide) error {
	// Validate Provide.DHT.Interval
	if !cfg.DHT.Interval.IsDefault() {
		interval := cfg.DHT.Interval.WithDefault(DefaultProvideDHTInterval)
		if interval > amino.DefaultProvideValidity {
			return fmt.Errorf("Provide.DHT.Interval (%v) must be less than or equal to DHT provider record validity (%v)", interval, amino.DefaultProvideValidity)
		}
		if interval < 0 {
			return fmt.Errorf("Provide.DHT.Interval must be non-negative, got %v", interval)
		}
	}

	// Validate MaxWorkers
	if !cfg.DHT.MaxWorkers.IsDefault() {
		maxWorkers := cfg.DHT.MaxWorkers.WithDefault(DefaultProvideDHTMaxWorkers)
		if maxWorkers <= 0 {
			return fmt.Errorf("Provide.DHT.MaxWorkers must be positive, got %d", maxWorkers)
		}
	}

	// Validate DedicatedPeriodicWorkers
	if !cfg.DHT.DedicatedPeriodicWorkers.IsDefault() {
		workers := cfg.DHT.DedicatedPeriodicWorkers.WithDefault(DefaultProvideDHTDedicatedPeriodicWorkers)
		if workers < 0 {
			return fmt.Errorf("Provide.DHT.DedicatedPeriodicWorkers must be non-negative, got %d", workers)
		}
	}

	// Validate DedicatedBurstWorkers
	if !cfg.DHT.DedicatedBurstWorkers.IsDefault() {
		workers := cfg.DHT.DedicatedBurstWorkers.WithDefault(DefaultProvideDHTDedicatedBurstWorkers)
		if workers < 0 {
			return fmt.Errorf("Provide.DHT.DedicatedBurstWorkers must be non-negative, got %d", workers)
		}
	}

	// Validate MaxProvideConnsPerWorker
	if !cfg.DHT.MaxProvideConnsPerWorker.IsDefault() {
		conns := cfg.DHT.MaxProvideConnsPerWorker.WithDefault(DefaultProvideDHTMaxProvideConnsPerWorker)
		if conns <= 0 {
			return fmt.Errorf("Provide.DHT.MaxProvideConnsPerWorker must be positive, got %d", conns)
		}
	}

	// Validate KeystoreBatchSize
	if !cfg.DHT.KeystoreBatchSize.IsDefault() {
		batchSize := cfg.DHT.KeystoreBatchSize.WithDefault(DefaultProvideDHTKeystoreBatchSize)
		if batchSize <= 0 {
			return fmt.Errorf("Provide.DHT.KeystoreBatchSize must be positive, got %d", batchSize)
		}
	}

	// Validate OfflineDelay
	if !cfg.DHT.OfflineDelay.IsDefault() {
		delay := cfg.DHT.OfflineDelay.WithDefault(DefaultProvideDHTOfflineDelay)
		if delay < 0 {
			return fmt.Errorf("Provide.DHT.OfflineDelay must be non-negative, got %v", delay)
		}
	}

	return nil
}
