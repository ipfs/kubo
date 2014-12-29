package net

import (
	"io"

	conn "github.com/jbenet/go-ipfs/p2p/net/conn"
	// swarm "github.com/jbenet/go-ipfs/p2p/net/swarm2"
	peer "github.com/jbenet/go-ipfs/p2p/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// ProtocolID is an identifier used to write protocol headers in streams.
type ProtocolID string

// These are the ProtocolIDs of the protocols running. It is useful
// to keep them in one place.
const (
	ProtocolTesting  ProtocolID = "/ipfs/testing"
	ProtocolBitswap  ProtocolID = "/ipfs/bitswap"
	ProtocolDHT      ProtocolID = "/ipfs/dht"
	ProtocolIdentify ProtocolID = "/ipfs/id"
	ProtocolDiag     ProtocolID = "/ipfs/diagnostics"
	ProtocolRelay    ProtocolID = "/ipfs/relay"
)

// MessageSizeMax is a soft (recommended) maximum for network messages.
// One can write more, as the interface is a stream. But it is useful
// to bunch it up into multiple read/writes when the whole message is
// a single, large serialized object.
const MessageSizeMax = 2 << 22 // 4MB

// Stream represents a bidirectional channel between two agents in
// the IPFS network. "agent" is as granular as desired, potentially
// being a "request -> reply" pair, or whole protocols.
// Streams are backed by SPDY streams underneath the hood.
type Stream interface {
	io.Reader
	io.Writer
	io.Closer

	// Conn returns the connection this stream is part of.
	Conn() Conn
}

// StreamHandler is the function protocols who wish to listen to
// incoming streams must implement.
type StreamHandler func(Stream)

type StreamHandlerMap map[ProtocolID]StreamHandler

// Conn is a connection to a remote peer. It multiplexes streams.
// Usually there is no need to use a Conn directly, but it may
// be useful to get information about the peer on the other side:
//  stream.Conn().RemotePeer()
type Conn interface {
	conn.PeerConn

	// NewStreamWithProtocol constructs a new Stream over this conn.
	NewStreamWithProtocol(pr ProtocolID) (Stream, error)
}

// Network is the interface IPFS uses for connecting to the world.
// It dials and listens for connections. it uses a Swarm to pool
// connnections (see swarm pkg, and peerstream.Swarm). Connections
// are encrypted with a TLS-like protocol.
type Network interface {
	Dialer
	io.Closer

	// SetHandler sets the protocol handler on the Network's Muxer.
	// This operation is threadsafe.
	SetHandler(ProtocolID, StreamHandler)

	// Protocols returns the list of protocols this network currently
	// has registered handlers for.
	Protocols() []ProtocolID

	// NewStream returns a new stream to given peer p.
	// If there is no connection to p, attempts to create one.
	// If ProtocolID is "", writes no header.
	NewStream(ProtocolID, peer.ID) (Stream, error)

	// BandwidthTotals returns the total number of bytes passed through
	// the network since it was instantiated
	BandwidthTotals() (uint64, uint64)

	// ListenAddresses returns a list of addresses at which this network listens.
	ListenAddresses() []ma.Multiaddr

	// InterfaceListenAddresses returns a list of addresses at which this network
	// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
	// use the known local interfaces.
	InterfaceListenAddresses() ([]ma.Multiaddr, error)

	// CtxGroup returns the network's contextGroup
	CtxGroup() ctxgroup.ContextGroup
}

// Dialer represents a service that can dial out to peers
// (this is usually just a Network, but other services may not need the whole
// stack, and thus it becomes easier to mock)
type Dialer interface {

	// Peerstore returns the internal peerstore
	// This is useful to tell the dialer about a new address for a peer.
	// Or use one of the public keys found out over the network.
	Peerstore() peer.Peerstore

	// LocalPeer returns the local peer associated with this network
	LocalPeer() peer.ID

	// DialPeer attempts to establish a connection to a given peer
	DialPeer(context.Context, peer.ID) error

	// ClosePeer closes the connection to a given peer
	ClosePeer(peer.ID) error

	// Connectedness returns a state signaling connection capabilities
	Connectedness(peer.ID) Connectedness

	// Peers returns the peers connected
	Peers() []peer.ID

	// Conns returns the connections in this Netowrk
	Conns() []Conn

	// ConnsToPeer returns the connections in this Netowrk for given peer.
	ConnsToPeer(p peer.ID) []Conn
}

// Connectedness signals the capacity for a connection with a given node.
// It is used to signal to services and other peers whether a node is reachable.
type Connectedness int

const (
	// NotConnected means no connection to peer, and no extra information (default)
	NotConnected Connectedness = iota

	// Connected means has an open, live connection to peer
	Connected

	// CanConnect means recently connected to peer, terminated gracefully
	CanConnect

	// CannotConnect means recently attempted connecting but failed to connect.
	// (should signal "made effort, failed")
	CannotConnect
)
