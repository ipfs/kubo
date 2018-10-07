package config

type Experiments struct {
	FilestoreEnabled     bool
	UrlstoreEnabled      bool
	ShardingEnabled      bool
	Libp2pStreamMounting bool
	P2pHttpProxy         bool
	QUIC                 bool
}
