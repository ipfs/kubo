package config

type Discovery struct {
	MDNS  MDNS
	Cjdns Cjdns
}

type MDNS struct {
	Enabled bool

	// Time in seconds between discovery rounds
	Interval int
}

type Cjdns struct {
	Enabled  bool
	Interval int
}
