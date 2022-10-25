package config

type Ipns struct {
	RepublishPeriod string
	RecordLifetime  string

	ResolveCacheSize int

	// Enable namesys pubsub (--enable-namesys-pubsub)
	UsePubsub Flag `json:",omitempty"`
}
