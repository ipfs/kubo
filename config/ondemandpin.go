package config

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-kad-dht/amino"
)

const (
	DefaultOnDemandPinReplicationTarget = 5
	DefaultOnDemandPinCheckInterval     = 10 * time.Minute

	// Must exceed amino.DefaultProvideValidity so stale DHT records expire
	// before unpin. The extra day covers check-interval skew.
	DefaultOnDemandPinUnpinGracePeriod = amino.DefaultProvideValidity + 24*time.Hour
)

type OnDemandPinning struct {
	// Minimum providers desired in the DHT (excluding self).
	ReplicationTarget OptionalInteger

	// How often the checker evaluates all registered CIDs.
	CheckInterval OptionalDuration

	// How long replication must stay above target before unpinning.
	UnpinGracePeriod OptionalDuration
}

// ValidateOnDemandPinningConfig rejects non-positive intervals/grace periods
// and ReplicationTarget < 1 (ticker panic / never pins, then unpins).
func ValidateOnDemandPinningConfig(cfg *OnDemandPinning) error {
	if target := cfg.ReplicationTarget.WithDefault(DefaultOnDemandPinReplicationTarget); target < 1 {
		return fmt.Errorf("OnDemandPinning.ReplicationTarget must be at least 1, got %d", target)
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
		return fmt.Errorf("on-demand pinning requires a routing system that can answer provider queries; Routing.Type=%q cannot", routingType)
	}
	return nil
}
