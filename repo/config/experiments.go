package config

type Experiments struct {
	FilestoreEnabled       bool
	ShardingEnabled        bool
	Libp2pStreamMounting   bool
	BitswapStrategyEnabled bool
	BitswapStrategy        string
	BitswapRRQRoundBurst   int
}
