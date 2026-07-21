package config

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-kad-dht/amino"
)

const (
	DefaultOnDemandPinReplicationTargetMin = 5
	DefaultOnDemandPinReplicationTargetMax = 7
	DefaultOnDemandPinCheckInterval        = 10 * time.Minute

	// Must exceed amino.DefaultProvideValidity so stale DHT records expire
	// before unpin. The extra day covers check-interval skew.
	DefaultOnDemandPinUnpinGracePeriod = amino.DefaultProvideValidity + 24*time.Hour
)

type OnDemandPinning struct {
	// Pin when fewer than this many providers are found in the DHT (excluding self).
	ReplicationTargetMin OptionalInteger

	// Start the unpin grace period only when more than this many providers are found.
	ReplicationTargetMax OptionalInteger

	// How often the checker evaluates all registered CIDs.
	CheckInterval OptionalDuration

	// How long replication must stay above max before unpinning; checker adds up to 2*CheckInterval of jitter.
	UnpinGracePeriod OptionalDuration
}

// ValidateOnDemandPinningConfig rejects invalid min/max and non-positive durations.
func ValidateOnDemandPinningConfig(cfg *OnDemandPinning) error {
	min := cfg.ReplicationTargetMin.WithDefault(DefaultOnDemandPinReplicationTargetMin)
	max := cfg.ReplicationTargetMax.WithDefault(DefaultOnDemandPinReplicationTargetMax)
	if min < 1 {
		return fmt.Errorf("OnDemandPinning.ReplicationTargetMin must be at least 1, got %d", min)
	}
	if max < min {
		return fmt.Errorf("OnDemandPinning.ReplicationTargetMax (%d) must be >= ReplicationTargetMin (%d)", max, min)
	}
	if interval := cfg.CheckInterval.WithDefault(DefaultOnDemandPinCheckInterval); interval <= 0 {
		return fmt.Errorf("OnDemandPinning.CheckInterval must be positive, got %v", interval)
	}
	if grace := cfg.UnpinGracePeriod.WithDefault(DefaultOnDemandPinUnpinGracePeriod); grace <= 0 {
		return fmt.Errorf("OnDemandPinning.UnpinGracePeriod must be positive, got %v", grace)
	}
	return nil
}

// ValidateOnDemandPinningRouting rejects Routing.Type=none.
// Otherwise the checker would pin every registered CID on a null router.
func ValidateOnDemandPinningRouting(routingType string) error {
	if routingType == "none" {
		return fmt.Errorf("on-demand pinning needs provider lookups; Routing.Type=%q is not usable", routingType)
	}
	return nil
}
