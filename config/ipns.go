package config

import (
	"math"
	"time"
)

const (
	DefaultIpnsMaxCacheTTL = time.Duration(math.MaxInt64)
)

type Ipns struct {
	RepublishPeriod string
	RecordLifetime  string

	ResolveCacheSize int

	// MaxCacheTTL is the maximum duration IPNS entries are valid in the cache.
	MaxCacheTTL *OptionalDuration `json:",omitempty"`

	// Enable namesys pubsub (--enable-namesys-pubsub)
	UsePubsub Flag `json:",omitempty"`
}
