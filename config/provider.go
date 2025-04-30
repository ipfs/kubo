package config

const (
	DefaultProviderWorkerCount = 64
)

// Provider configuration describes how NEW CIDs are announced the moment they are created.
// For periodical reprovide configuration, see Reprovider.*
type Provider struct {
	Strategy    *OptionalString  `json:",omitempty"` // Unused, you are likely looking for Reprovider.Strategy instead
	WorkerCount *OptionalInteger `json:",omitempty"` // Number of concurrent provides allowed, 0 means unlimited
}
