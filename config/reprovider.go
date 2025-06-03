package config

import "time"

const (
	DefaultReproviderInterval = time.Hour * 22 // https://github.com/ipfs/kubo/pull/9326
	DefaultReproviderStrategy = "all"
)

// Reprovider configuration describes how CID from local datastore are periodically re-announced to routing systems.
// For provide behavior of ad-hoc or newly created CIDs and their first-time announcement, see Provider.*
type Reprovider struct {
	Interval *OptionalDuration `json:",omitempty"` // Time period to reprovide locally stored objects to the network
	Strategy *OptionalString   `json:",omitempty"` // Which keys to announce
}
