package config

type Discovery struct {
	MDNS      MDNS
	Broadcast Broadcast
}

type MDNS struct {
	Enabled bool

	// Time in seconds between discovery rounds
	Interval int
}

type Broadcast struct {
	Enabled bool

	// Time in seconds between broadcast messages
	Interval int
}
