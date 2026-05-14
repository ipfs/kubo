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

	// DefaultProvideBloomFPRate is the target false positive rate for the
	// bloom filter used by +unique and +entities reprovide cycles and
	// fast-provide-dag walks. Expressed as 1/N (one false positive per N
	// lookups). At ~1 in 4.75M (~0.00002%) each CID costs ~4 bytes before
	// ipfs/bbloom's power-of-two rounding.
	//
	// Kubo owns this default independently of boxo/dag/walker; the two
	// values may diverge over time without coordination.
	DefaultProvideBloomFPRate = 4_750_000

	// MinProvideBloomFPRate is the smallest accepted Provide.BloomFPRate.
	// Below 1 in 1M the bloom filter becomes lossy enough to drop a
	// meaningful fraction of CIDs from each reprovide cycle (e.g. at
	// rate=10_000 a 100M-CID repo skips ~10K CIDs per cycle).
	MinProvideBloomFPRate = 1_000_000

	// DHT provider defaults
	DefaultProvideDHTInterval                  = 22 * time.Hour // https://github.com/ipfs/kubo/pull/9326
	DefaultProvideDHTMaxWorkers                = 16             // Unified default for both sweep and legacy providers
	DefaultProvideDHTSweepEnabled              = true
	DefaultProvideDHTResumeEnabled             = true
	DefaultProvideDHTDedicatedPeriodicWorkers  = 2
	DefaultProvideDHTDedicatedBurstWorkers     = 1
	DefaultProvideDHTMaxProvideConnsPerWorker  = 20
	DefaultProvideDHTKeystoreBatchSize         = 1 << 14 // ~544 KiB per batch (1 multihash = 34 bytes)
	DefaultProvideDHTOfflineDelay              = 2 * time.Hour
	DefaultProvideDHTSendProviderRecordTimeout = 10 * time.Second

	// DefaultFastProvideTimeout is the maximum time allowed for fast-provide operations.
	// Prevents hanging on network issues when providing root CID.
	// 10 seconds is sufficient for DHT operations with sweep provider or accelerated client.
	DefaultFastProvideTimeout = 10 * time.Second
)

type ProvideStrategy int

const (
	ProvideStrategyAll ProvideStrategy = 1 << iota
	ProvideStrategyPinned
	ProvideStrategyRoots
	ProvideStrategyMFS
	ProvideStrategyUnique   // bloom filter cross-DAG deduplication
	ProvideStrategyEntities // entity-aware traversal (implies Unique)
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

	// BloomFPRate sets the target false positive rate of the bloom filter
	// used by Provide.Strategy modifiers +unique and +entities (and the
	// matching fast-provide-dag walk). Expressed as 1/N (one false
	// positive per N lookups), so higher N means lower FP rate but more
	// memory per CID. Only takes effect when Provide.Strategy includes
	// +unique or +entities.
	//
	// Default: DefaultProvideBloomFPRate
	BloomFPRate *OptionalInteger `json:",omitempty"`

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
	// reprovide operations over time.
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

	// SendProviderRecordTimeout sets the per-peer timeout applied to a single
	// ADD_PROVIDER RPC. A peer that accepts the libp2p stream but never reads
	// the request must not pin a provide worker goroutine indefinitely; this
	// timeout bounds the wait (sweep mode only).
	// Default: DefaultProvideDHTSendProviderRecordTimeout
	SendProviderRecordTimeout *OptionalDuration `json:",omitempty"`

	// ResumeEnabled controls whether the provider resumes from its previous state on restart.
	// When enabled, the provider persists its reprovide cycle state and provide queue to the datastore,
	// and restores them on restart. When disabled, the provider starts fresh on each restart.
	// Default: true
	ResumeEnabled Flag `json:",omitempty"`
}

func ParseProvideStrategy(s string) (ProvideStrategy, error) {
	var strategy ProvideStrategy
	for part := range strings.SplitSeq(s, "+") {
		switch part {
		case "all", "flat":
			strategy |= ProvideStrategyAll
		case "":
			// empty string (default config) maps to "all",
			// but empty tokens from splitting (e.g. "pinned+") are invalid
			if s == "" {
				strategy |= ProvideStrategyAll
			} else {
				return 0, fmt.Errorf("invalid provide strategy: empty token in %q", s)
			}
		case "pinned":
			strategy |= ProvideStrategyPinned
		case "roots":
			strategy |= ProvideStrategyRoots
		case "mfs":
			strategy |= ProvideStrategyMFS
		case "unique":
			strategy |= ProvideStrategyUnique
		case "entities":
			strategy |= ProvideStrategyEntities | ProvideStrategyUnique
		default:
			return 0, fmt.Errorf("unknown provide strategy token: %q in %q", part, s)
		}
	}
	// "all" provides every block and cannot be combined with selective strategies
	if strategy&ProvideStrategyAll != 0 && strategy != ProvideStrategyAll {
		return 0, fmt.Errorf("\"all\" strategy cannot be combined with other strategies in %q", s)
	}
	// +unique/+entities require a base strategy that walks DAGs (pinned and/or mfs)
	wantsDedup := strategy&(ProvideStrategyUnique|ProvideStrategyEntities) != 0
	if wantsDedup {
		walksDAGs := strategy&(ProvideStrategyPinned|ProvideStrategyMFS) != 0
		if !walksDAGs {
			return 0, fmt.Errorf("+unique/+entities must combine with pinned and/or mfs in %q", s)
		}
		if strategy&ProvideStrategyRoots != 0 {
			return 0, fmt.Errorf("+unique/+entities is incompatible with roots in %q", s)
		}
	}
	return strategy, nil
}

// MustParseProvideStrategy is like ParseProvideStrategy but panics on error.
// Use with strategy strings that have already been validated at startup.
func MustParseProvideStrategy(s string) ProvideStrategy {
	strategy, err := ParseProvideStrategy(s)
	if err != nil {
		panic(err)
	}
	return strategy
}

// ValidateProvideConfig validates the Provide configuration according to DHT requirements.
func ValidateProvideConfig(cfg *Provide) error {
	// Validate Provide.Strategy
	strategy := cfg.Strategy.WithDefault(DefaultProvideStrategy)
	if _, err := ParseProvideStrategy(strategy); err != nil {
		return fmt.Errorf("Provide.Strategy: %w", err)
	}

	// Validate Provide.BloomFPRate
	if !cfg.BloomFPRate.IsDefault() {
		rate := cfg.BloomFPRate.WithDefault(DefaultProvideBloomFPRate)
		if rate < MinProvideBloomFPRate {
			return fmt.Errorf("Provide.BloomFPRate must be >= %d (1 in 1M), got %d", MinProvideBloomFPRate, rate)
		}
	}

	// Validate Provide.DHT.Interval
	if !cfg.DHT.Interval.IsDefault() {
		interval := cfg.DHT.Interval.WithDefault(DefaultProvideDHTInterval)
		if interval > amino.DefaultProvideValidity {
			return fmt.Errorf("Provide.DHT.Interval (%v) must be less than or equal to DHT provider record validity (%v)", interval, amino.DefaultProvideValidity)
		}
		if interval < 0 {
			return fmt.Errorf("Provide.DHT.Interval must be non-negative, got %v", interval)
		}
		// Provide.DHT.Interval=0 used to disable the entire provide system as a
		// side effect. It now disables only the periodic reprovide schedule:
		// new CIDs still announce via fast-provide-root and 'ipfs provide once'.
		// Operators upgrading from earlier kubo versions must opt in to one of
		// the two semantics by setting Provide.Enabled explicitly:
		//   - Provide.Enabled=false fully disables providing (the old behaviour).
		//   - Provide.Enabled=true keeps ad-hoc providing while disabling the
		//     periodic reprovide schedule.
		if interval == 0 && cfg.Enabled == Default {
			return fmt.Errorf("Provide.DHT.Interval=0 no longer disables the provide system on its own; set Provide.Enabled explicitly: " +
				"Provide.Enabled=false to fully disable providing, or Provide.Enabled=true to keep ad-hoc 'ipfs provide once' " +
				"and fast-provide-root working while skipping the periodic reprovide schedule")
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

	// Validate SendProviderRecordTimeout
	if !cfg.DHT.SendProviderRecordTimeout.IsDefault() {
		timeout := cfg.DHT.SendProviderRecordTimeout.WithDefault(DefaultProvideDHTSendProviderRecordTimeout)
		if timeout <= 0 {
			return fmt.Errorf("Provide.DHT.SendProviderRecordTimeout must be positive, got %v", timeout)
		}
	}

	return nil
}

// ShouldProvideForStrategy determines if content should be provided based on the provide strategy
// and content characteristics (pinned status, root status, MFS status).
func ShouldProvideForStrategy(strategy ProvideStrategy, isPinned bool, isPinnedRoot bool, isMFS bool) bool {
	if strategy&ProvideStrategyAll != 0 {
		// 'all' strategy: always provide
		return true
	}

	// For combined strategies, check each component
	if strategy&ProvideStrategyPinned != 0 && isPinned {
		return true
	}
	if strategy&ProvideStrategyRoots != 0 && isPinnedRoot {
		return true
	}
	if strategy&ProvideStrategyMFS != 0 && isMFS {
		return true
	}

	return false
}
