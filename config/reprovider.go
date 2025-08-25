package config

import (
	"strings"
	"time"
)

const (
	DefaultReproviderInterval = time.Hour * 22 // https://github.com/ipfs/kubo/pull/9326
	DefaultReproviderStrategy = "all"

	DefaultReproviderSweepEnabled                  = false
	DefaultReproviderSweepMaxWorkers               = 4
	DefaultReproviderSweepDedicatedPeriodicWorkers = 2
	DefaultReproviderSweepDedicatedBurstWorkers    = 1
	DefaultReproviderSweepMaxProvideConnsPerWorker = 16
	DefaultReproviderSweepKeyStoreBatchSize        = 1 << 14 // ~544 KiB per batch (1 multihash = 34 bytes)
)

type ReproviderStrategy int

const (
	ReproviderStrategyAll ReproviderStrategy = 1 << iota
	ReproviderStrategyPinned
	ReproviderStrategyRoots
	ReproviderStrategyMFS
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

	KeyStoreGCInterval *OptionalDuration // Interval for garbage collection in KeyStore.
	KeyStoreBatchSize  *OptionalInteger  // Number of multihashes to keep in memory when gc'ing the KeyStore.
}

func ParseReproviderStrategy(s string) ReproviderStrategy {
	var strategy ReproviderStrategy
	for _, part := range strings.Split(s, "+") {
		switch part {
		case "all", "flat", "": // special case, does not mix with others ("flat" is deprecated, maps to "all")
			return ReproviderStrategyAll
		case "pinned":
			strategy |= ReproviderStrategyPinned
		case "roots":
			strategy |= ReproviderStrategyRoots
		case "mfs":
			strategy |= ReproviderStrategyMFS
		}
	}
	return strategy
}
