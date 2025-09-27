package config

// Reprovider configuration describes how CID from local datastore are periodically re-announced to routing systems.
// For provide behavior of ad-hoc or newly created CIDs and their first-time announcement, see Provide.*
//
// Deprecated: use Provide instead. This will be removed in a future release.
type Reprovider struct {
	// Deprecated: use Provide.DHT.Interval instead. This will be removed in a future release.
	Interval *OptionalDuration `json:",omitempty"`

	// Deprecated: use Provide.Strategy instead. This will be removed in a future release.
	Strategy *OptionalString `json:",omitempty"`
}
