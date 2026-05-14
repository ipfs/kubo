package config

type Discovery struct {
	MDNS MDNS
}

type MDNS struct {
	Enabled bool
}
