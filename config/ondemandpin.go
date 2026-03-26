package config

import "time"

const (
	DefaultOnDemandPinReplicationTarget = 5
	DefaultOnDemandPinCheckInterval     = 10 * time.Minute
	DefaultOnDemandPinUnpinGracePeriod  = 24 * time.Hour
)

type OnDemandPinning struct {
	// Minimum providers desired in the DHT (excluding self).
	ReplicationTarget OptionalInteger

	// How often the checker evaluates all registered CIDs.
	CheckInterval OptionalDuration

	// How long replication must stay above target before unpinning.
	UnpinGracePeriod OptionalDuration
}
