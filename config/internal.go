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

	// BroadcastReductioEnabled enables or disables broadcast reduction logic.
	// If broadcast reduction logic is disabled, then the other Broadcast
	// configuration items are ignored. Setting this to false restores the
	// previous broadcast behavior of broadcasting to all peers. Default is
	// DefaultBroadcastReductionEnabled.
	BroadcastReductionEnabled Flag `json:",omitempty"`
	// BroadcastLimitPeers sets a hard limit on the number of peers to send
	// broadcasts to. A value of 0 means no broadcasts are sent. A value of -1
	// means there is no limit. Default is -1. Default is
	// DefaultBroadcastLimitPeers.
	BroadcastLimitPeers OptionalInteger `json:",omitempty"`
	// BroadcastReduceAll enables or disables broadcast reduction for peers on
	// the local network and peers configured for peering. If false, than
	// always broadcast to peers on the local network and peers configured for
	// peering. If true, apply broadcast reduction to all peers without special
	// consideration for local and peering peers. Default is
	// DefaultBroadcastReduceAll.
	BroadcastReduceAll Flag `json:",omitempty"`
	// BroadcastSendRandomPeers sets the number of peers to broadcast to
	// anyway, even though broadcast reduction logic has determined that they
	// are not broadcast targets. Setting this to a non-zero value ensures at
	// least this number of random peers receives a broadcast. This may be
	// helpful in cases where peers that are not receiving broadcasts my have
	// wanted blocks. Default is DefaultBroadcastSendRandomPeers.
	BroadcastSendRandomPeers OptionalInteger `json:",omitempty"`
	// BroadcastSendWithPending, if true, enables sending broadcasts to any
	// peers that already have a pending message to send. This sends broadcasts
	// to many more peers, but in a way that does not increase the number of
	// separate broadcast messages. There is still the increased cost of the
	// recipients having to process and respond to the broadcasts. Default is
	// DefaultBroadcastSendWithPending.
	BroadcastSendWithPending Flag `json:",omitempty"`
}

const (
	DefaultBroadcastReductionEnabled = true
	DefaultBroadcastLimitPeers       = -1
	DefaultBroadcastReduceAll        = false
	DefaultBroadcastSendRandomPeers  = 0
	DefaultBroadcastSendWithPending  = false
)
