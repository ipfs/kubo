package config

type Discovery struct {
	MDNS MDNS

	//Routing sets default daemon routing mode.
	Routing string
}

type MDNS struct {
	Enabled bool

	// Time in seconds between discovery rounds
	Interval int
}
