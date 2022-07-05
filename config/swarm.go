package config

import rcmgr "github.com/libp2p/go-libp2p-resource-manager"

type SwarmConfig struct {
	// AddrFilters specifies a set libp2p addresses that we should never
	// dial or receive connections from.
	AddrFilters []string

	// DisableBandwidthMetrics disables recording of bandwidth metrics for a
	// slight reduction in memory usage. You probably don't need to set this
	// flag.
	DisableBandwidthMetrics bool

	// DisableNatPortMap turns off NAT port mapping (UPnP, etc.).
	DisableNatPortMap bool

	// DisableRelay explicitly disables the relay transport.
	//
	// Deprecated: This flag is deprecated and is overridden by
	// `Swarm.Transports.Relay` if specified.
	DisableRelay bool `json:",omitempty"`

	// EnableRelayHop makes this node act as a public relay v1
	//
	// Deprecated: The circuit v1 protocol is deprecated.
	// Use `Swarm.RelayService` to configure the circuit v2 relay.
	EnableRelayHop bool `json:",omitempty"`

	// EnableAutoRelay enables the "auto relay user" feature.
	// Node will find and use advertised public relays when it determines that
	// it's not reachable from the public internet.
	//
	// Deprecated: This flag is deprecated and is overridden by
	// `Swarm.RelayClient.Enabled` if specified.
	EnableAutoRelay bool `json:",omitempty"`

	// RelayClient controls the client side of "auto relay" feature.
	// When enabled, the node will use relays if it is not publicly reachable.
	RelayClient RelayClient

	// RelayService.* controls the "relay service".
	// When enabled, node will provide a limited relay service to other peers.
	RelayService RelayService

	// EnableHolePunching enables the hole punching service.
	EnableHolePunching Flag `json:",omitempty"`

	// Transports contains flags to enable/disable libp2p transports.
	Transports Transports

	// ConnMgr configures the connection manager.
	ConnMgr ConnMgr

	// ResourceMgr configures the libp2p Network Resource Manager
	ResourceMgr ResourceMgr
}

type RelayClient struct {
	// Enables the auto relay feature: will use relays if it is not publicly reachable.
	Enabled Flag `json:",omitempty"`

	// StaticRelays configures static relays to use when this node is not
	// publicly reachable. If set, auto relay will not try to find any
	// other relay servers.
	StaticRelays []string `json:",omitempty"`
}

// RelayService configures the resources of the circuit v2 relay.
// For every field a reasonable default will be defined in go-ipfs.
type RelayService struct {
	// Enables the limited relay service for other peers (circuit v2 relay).
	Enabled Flag `json:",omitempty"`

	// ConnectionDurationLimit is the time limit before resetting a relayed connection.
	ConnectionDurationLimit *OptionalDuration `json:",omitempty"`
	// ConnectionDataLimit is the limit of data relayed (on each direction) before resetting the connection.
	ConnectionDataLimit *OptionalInteger `json:",omitempty"`

	// ReservationTTL is the duration of a new (or refreshed reservation).
	ReservationTTL *OptionalDuration `json:",omitempty"`

	// MaxReservations is the maximum number of active relay slots.
	MaxReservations *OptionalInteger `json:",omitempty"`
	// MaxCircuits is the maximum number of open relay connections for each peer; defaults to 16.
	MaxCircuits *OptionalInteger `json:",omitempty"`
	// BufferSize is the size of the relayed connection buffers.
	BufferSize *OptionalInteger `json:",omitempty"`

	// MaxReservationsPerPeer is the maximum number of reservations originating from the same peer.
	MaxReservationsPerPeer *OptionalInteger `json:",omitempty"`
	// MaxReservationsPerIP is the maximum number of reservations originating from the same IP address.
	MaxReservationsPerIP *OptionalInteger `json:",omitempty"`
	// MaxReservationsPerASN is the maximum number of reservations origination from the same ASN.
	MaxReservationsPerASN *OptionalInteger `json:",omitempty"`
}

type Transports struct {
	// Network specifies the base transports we'll use for dialing. To
	// listen on a transport, add the transport to your Addresses.Swarm.
	Network struct {
		// All default to on.
		QUIC      Flag `json:",omitempty"`
		TCP       Flag `json:",omitempty"`
		Websocket Flag `json:",omitempty"`
		Relay     Flag `json:",omitempty"`
	}

	// Security specifies the transports used to encrypt insecure network
	// transports.
	Security struct {
		// Defaults to 100.
		TLS Priority `json:",omitempty"`
		// Defaults to 200.
		SECIO Priority `json:",omitempty"`
		// Defaults to 300.
		Noise Priority `json:",omitempty"`
	}

	// Multiplexers specifies the transports used to multiplex multiple
	// connections over a single duplex connection.
	Multiplexers struct {
		// Defaults to 100.
		Yamux Priority `json:",omitempty"`
		// Defaults to 200.
		Mplex Priority `json:",omitempty"`
	}
}

// ConnMgr defines configuration options for the libp2p connection manager
type ConnMgr struct {
	Type        string
	LowWater    int
	HighWater   int
	GracePeriod string
}

// ResourceMgr defines configuration options for the libp2p Network Resource Manager
// <https://github.com/libp2p/go-libp2p-resource-manager#readme>
type ResourceMgr struct {
	// Enables the Network Resource Manager feature, default to on.
	Enabled Flag               `json:",omitempty"`
	Limits  *rcmgr.LimitConfig `json:",omitempty"`
	// A list of multiaddrs that can bypass normal system limits (but are still
	// limited by the allowlist scope). Convenience config around
	// https://pkg.go.dev/github.com/libp2p/go-libp2p-resource-manager#Allowlist.Add
	Allowlist []string `json:",omitempty"`
}

const (
	ResourceMgrSystemScope         = "system"
	ResourceMgrTransientScope      = "transient"
	ResourceMgrServiceScopePrefix  = "svc:"
	ResourceMgrProtocolScopePrefix = "proto:"
	ResourceMgrPeerScopePrefix     = "peer:"
)
