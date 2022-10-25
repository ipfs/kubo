package config

type Discovery struct {
	MDNS MDNS
}

type MDNS struct {
	Enabled bool

	// DEPRECATED: the time between discovery rounds is no longer configurable
	// See: https://github.com/ipfs/go-ipfs/pull/9048#discussion_r906814717
	Interval *OptionalInteger `json:",omitempty"`
}
