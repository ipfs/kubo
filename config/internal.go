package config

type Internal struct {
	// All marked as omitempty since we are expecting to make changes to all subcomponents of Internal
	Bitswap                     *InternalBitswap  `json:",omitempty"`
	UnixFSShardingSizeThreshold *OptionalString   `json:",omitempty"` // moved to Import.UnixFSHAMTDirectorySizeThreshold
	Libp2pForceReachability     *OptionalString   `json:",omitempty"`
	BackupBootstrapInterval     *OptionalDuration `json:",omitempty"`
}

type InternalBitswap struct {
	TaskWorkerCount             OptionalInteger
	EngineBlockstoreWorkerCount OptionalInteger
	EngineTaskWorkerCount       OptionalInteger
	MaxOutstandingBytesPerPeer  OptionalInteger
	ProviderSearchDelay         OptionalDuration
	ProviderSearchMaxResults    OptionalInteger
	WantHaveReplaceSize         OptionalInteger

	BroadcastNoReduction     bool `json:",omitempty"`
	BroadcastLimitPeers      int  `json:",omitempty"`
	BroadcastReduceLocal     bool `json:",omitempty"`
	BroadcastSendSkipped     int  `json:",omitempty"`
	BroadcastSendWithPending bool `json:",omitempty"`
}
