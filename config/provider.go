package config

const (
	DefaultProviderEnabled     = true
	DefaultProviderWorkerCount = 16
)

// Provider configuration describes how NEW CIDs are announced the moment they are created.
// For periodical reprovide configuration, see Provide.*
//
// Deprecated: use Provide instead. This will be removed in a future release.
type Provider struct {
	Enabled     Flag             `json:",omitempty"`
	Strategy    *OptionalString  `json:",omitempty"` // Unused, you are likely looking for Provide.Strategy instead
	WorkerCount *OptionalInteger `json:",omitempty"` // Number of concurrent provides allowed, 0 means unlimited
}
