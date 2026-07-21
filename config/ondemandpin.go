package config

import (
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
