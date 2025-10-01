package config

const (
	// DefaultMFSNoFlushLimit is the default limit for consecutive unflushed MFS operations
	DefaultMFSNoFlushLimit = 256
)

type Internal struct {
	// All marked as omitempty since we are expecting to make changes to all subcomponents of Internal
	Bitswap                     *InternalBitswap  `json:",omitempty"`
	UnixFSShardingSizeThreshold *OptionalString   `json:",omitempty"` // moved to Import.UnixFSHAMTDirectorySizeThreshold
	Libp2pForceReachability     *OptionalString   `json:",omitempty"`
	BackupBootstrapInterval     *OptionalDuration `json:",omitempty"`
	// MFSNoFlushLimit controls the maximum number of consecutive
	// MFS operations allowed with --flush=false before requiring a manual flush.
	// This prevents unbounded memory growth and ensures data consistency.
	// Set to 0 to disable limiting (old behavior, may cause high memory usage)
	// This is an EXPERIMENTAL feature and may change or be removed in future releases.
	// See https://github.com/ipfs/kubo/issues/10842
	MFSNoFlushLimit *OptionalInteger `json:",omitempty"`
}

type InternalBitswap struct {
	TaskWorkerCount             OptionalInteger
	EngineBlockstoreWorkerCount OptionalInteger
	EngineTaskWorkerCount       OptionalInteger
	MaxOutstandingBytesPerPeer  OptionalInteger
	ProviderSearchDelay         OptionalDuration
	ProviderSearchMaxResults    OptionalInteger
	WantHaveReplaceSize         OptionalInteger
	BroadcastControl            *BitswapBroadcastControl
}

type BitswapBroadcastControl struct {
	// EnableEnables or disables broadcast control functionality. Setting this
	// to false disables broadcast control functionality and restores the
	// previous broadcast behavior of sending broadcasts to all peers. When
	// disabled, all other BroadcastControl configuration items are ignored.
	// Default is [DefaultBroadcastControlEnable].
	Enable Flag `json:",omitempty"`
	// MaxPeers sets a hard limit on the number of peers to send broadcasts to.
	// A value of 0 means no broadcasts are sent. A value of -1 means there is
	// no limit. Default is [DefaultBroadcastControlMaxPeers].
	MaxPeers OptionalInteger `json:",omitempty"`
	// LocalPeers enables or disables broadcast control for peers on the local
	// network. If false, than always broadcast to peers on the local network.
	// If true, apply broadcast control to local peers. Default is
	// [DefaultBroadcastControlLocalPeers].
	LocalPeers Flag `json:",omitempty"`
	// PeeredPeers enables or disables broadcast reduction for peers configured
	// for peering. If false, than always broadcast to peers configured for
	// peering. If true, apply broadcast reduction to peered peers. Default is
	// [DefaultBroadcastControlPeeredPeers].
	PeeredPeers Flag `json:",omitempty"`
	// MaxRandomPeers is the number of peers to broadcast to anyway, even
	// though broadcast reduction logic has determined that they are not
	// broadcast targets. Setting this to a non-zero value ensures at least
	// this number of random peers receives a broadcast. This may be helpful in
	// cases where peers that are not receiving broadcasts my have wanted
	// blocks. Default is [DefaultBroadcastControlMaxRandomPeers].
	MaxRandomPeers OptionalInteger `json:",omitempty"`
	// SendToPendingPeers enables or disables sending broadcasts to any peers
	// to which there is a pending message to send. When enabled, this sends
	// broadcasts to many more peers, but does so in a way that does not
	// increase the number of separate broadcast messages. There is still the
	// increased cost of the recipients having to process and respond to the
	// broadcasts. Default is [DefaultBroadcastControlSendToPendingPeers].
	SendToPendingPeers Flag `json:",omitempty"`
}

const (
	DefaultBroadcastControlEnable             = true  // Enabled
	DefaultBroadcastControlMaxPeers           = -1    // Unlimited
	DefaultBroadcastControlLocalPeers         = false // No control of local
	DefaultBroadcastControlPeeredPeers        = false // No control of peered
	DefaultBroadcastControlMaxRandomPeers     = 0     // No randoms
	DefaultBroadcastControlSendToPendingPeers = false // Disabled
)
