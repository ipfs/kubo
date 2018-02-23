package namesys

import (
	"time"
)

// ResolveOpts specifies options for resolving an IPNS path
type ResolveOpts struct {
	// Recursion depth limit
	Depth uint
	// The number of IPNS records to retrieve from the DHT
	// (the best record is selected from this set)
	DhtRecordCount uint
	// The amount of time to wait for DHT records to be fetched
	// and verified. A zero value indicates that there is no explicit
	// timeout (although there is an implicit timeout due to dial
	// timeouts within the DHT)
	DhtTimeout time.Duration
}

// DefaultResolveOpts returns the default options for resolving
// an IPNS path
func DefaultResolveOpts() *ResolveOpts {
	return &ResolveOpts{
		Depth:          DefaultDepthLimit,
		DhtRecordCount: 16,
		DhtTimeout:     time.Minute,
	}
}
