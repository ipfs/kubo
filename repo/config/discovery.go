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
	Enabled         bool
	DialTimeout     int
	Interval        int    // 10m0s
	RefreshInterval int    // 24h0m0s
	AdminAddress    string // /ip4/127.0.0.1/udp/11234
	Password        string // NONE
}
