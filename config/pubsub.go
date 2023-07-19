package config

const (
	// LastSeenMessagesStrategy is a strategy that calculates the TTL countdown
	// based on the last time a Pubsub message is seen. This means that if a message
	// is received and then seen again within the specified TTL window, it
	// won't be emitted until the TTL countdown expires from the last time the
	// message was seen.
	LastSeenMessagesStrategy = "last-seen"

	// FirstSeenMessagesStrategy is a strategy that calculates the TTL
	// countdown based on the first time a Pubsub message is seen. This means that if
	// a message is received and then seen again within the specified TTL
	// window, it won't be emitted.
	FirstSeenMessagesStrategy = "first-seen"

	// DefaultSeenMessagesStrategy is the strategy that is used by default if
	// no Pubsub.SeenMessagesStrategy is specified.
	DefaultSeenMessagesStrategy = LastSeenMessagesStrategy
)

type PubsubConfig struct {
	// Router can be either floodsub (legacy) or gossipsub (new and
	// backwards compatible).
	Router string

	// DisableSigning disables message signing. Message signing is *enabled*
	// by default.
	DisableSigning bool

	// Enable pubsub (--enable-pubsub-experiment)
	Enabled Flag `json:",omitempty"`

	// SeenMessagesTTL is a value that controls the time window within which
	// duplicate messages will be identified and won't be emitted.
	SeenMessagesTTL *OptionalDuration `json:",omitempty"`

	// SeenMessagesStrategy is a setting that determines how the time-to-live
	// (TTL) countdown for deduplicating messages is calculated.
	SeenMessagesStrategy *OptionalString `json:",omitempty"`
}
