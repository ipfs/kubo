package config

// Provider configuration describes how NEW CIDs are announced the moment they are created.
// For periodical reprovide configuration, see Provide.*
//
// Deprecated: use Provide instead. This will be removed in a future release.
type Provider struct {
	// Deprecated: use Provide.Enabled instead. This will be removed in a future release.
	Enabled Flag `json:",omitempty"`

	// Deprecated: unused, you are likely looking for Provide.Strategy instead. This will be removed in a future release.
	Strategy *OptionalString `json:",omitempty"`

	// Deprecated: use Provide.DHT.MaxWorkers instead. This will be removed in a future release.
	WorkerCount *OptionalInteger `json:",omitempty"`
}
