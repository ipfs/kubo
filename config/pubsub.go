package config

type PubsubConfig struct {
	// Router can be either floodsub (legacy) or gossipsub (new and
	// backwards compatible).
	Router string

	// DisableSigning disables message signing. Message signing is *enabled*
	// by default.
	DisableSigning bool

	// StrictSignatureVerification enables strict signature verification.
	// When enabled, unsigned messages will be rejected. Eventually, this
	// will be made the default and this option will disappear. Once this
	// happens, networks will either need to completely disable or
	// completely enable message signing.
	StrictSignatureVerification bool
}
