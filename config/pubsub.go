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

	// SeenMessagesTTL configures the duration after which a previously seen
	// message ID can be forgotten about.
	SeenMessagesTTL *OptionalDuration `json:",omitempty"`
}
