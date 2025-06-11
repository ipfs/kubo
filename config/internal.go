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

	BroadcastReductionEnabled Flag            `json:",omitempty"`
	BroadcastLimitPeers       OptionalInteger `json:",omitempty"`
	BroadcastReduceLocal      Flag            `json:",omitempty"`
	BroadcastSendSkipped      OptionalInteger `json:",omitempty"`
	BroadcastSendWithPending  Flag            `json:",omitempty"`
}

const (
	DefaultBroadcastReductionEnabled = true
	DefaultBroadcastLimitPeers       = 0
	DefaultBroadcastReduceLocal      = false
	DefaultBroadcastSendSkipped      = 0
	DefaultBroadcastSendWithPending  = false
)
