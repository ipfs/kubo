package config

import (
	"strings"
	"time"
)

const (
	DefaultReproviderInterval = time.Hour * 22 // https://github.com/ipfs/kubo/pull/9326
	DefaultReproviderStrategy = "all"
)

type ReproviderStrategy int

const (
	ReproviderStrategyAll    ReproviderStrategy = 1 << iota // 1  (0b00001)
	ReproviderStrategyFlat                                  // 2  (0b00010)
	ReproviderStrategyPinned                                // 4  (0b00100)
	ReproviderStrategyRoots                                 // 8  (0b01000)
	ReproviderStrategyMFS                                   // 16 (0b10000)
)

// Reprovider configuration describes how CID from local datastore are periodically re-announced to routing systems.
// For provide behavior of ad-hoc or newly created CIDs and their first-time announcement, see Provider.*
type Reprovider struct {
	Interval *OptionalDuration `json:",omitempty"` // Time period to reprovide locally stored objects to the network
	Strategy *OptionalString   `json:",omitempty"` // Which keys to announce
}

func ParseReproviderStrategy(s string) ReproviderStrategy {
	var strategy ReproviderStrategy
	for _, part := range strings.Split(s, "+") {
		switch part {
		case "all", "": // special case, does not mix with others
			return ReproviderStrategyAll
		case "flat":
			strategy |= ReproviderStrategyFlat
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
