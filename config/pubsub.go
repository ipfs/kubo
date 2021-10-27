package config

type PubsubConfig struct {
	// Router can be either floodsub (legacy) or gossipsub (new and
	// backwards compatible).
	Router string

	// DisableSigning disables message signing. Message signing is *enabled*
	// by default.
	DisableSigning bool

	// Enable pubsub (--enable-pubsub-experiment)
	Enabled Flag `json:",omitempty"`
}
