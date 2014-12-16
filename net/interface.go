package net

import (
	"io"

	conn "github.com/jbenet/go-ipfs/net/conn"
	swarm "github.com/jbenet/go-ipfs/net/swarm2"
	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

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

// Conn is a connection to a remote peer. It multiplexes streams.
// Usually there is no need to use a Conn directly, but it may
// be useful to get information about the peer on the other side:
//  stream.Conn().RemotePeer()
type Conn interface {
	conn.PeerConn

	// NewStream constructs a new Stream directly connected to p.
	NewStream(p peer.Peer) (Stream, error)
}

// Mux provides simple stream multixplexing.
// It helps you precisely when:
//  * You have many streams
//  * You have function handlers
//
// It contains the handlers for each protocol accepted.
// It dispatches handlers for streams opened by remote peers.
//
// We use a totally ad-hoc encoding:
//   <1 byte length in bytes><string name>
// So "bitswap" is 0x0762697473776170
//
// NOTE: only the dialer specifies this muxing line.
// This is because we're using Streams :)
//
// WARNING: this datastructure IS NOT threadsafe.
// do not modify it once the network is using it.
type Mux struct {
	Default  StreamHandler // handles unknown protocols.
	Handlers map[string]StreamHandler
}

// Network is the interface IPFS uses for connecting to the world.
// It dials and listens for connections. it uses a Swarm to pool
// connnections (see swarm pkg, and peerstream.Swarm). Connections
// are encrypted with a TLS-like protocol.
type Network interface {
	Dialer
	io.Closer

	// NewStream returns a new stream to given peer p.
	// If there is no connection to p, attempts to create one.
	NewStream(p peer.Peer) (Stream, error)

	// Swarm returns the connection Swarm
	Swarm() *swarm.Swarm

	// BandwidthTotals returns the total number of bytes passed through
	// the network since it was instantiated
	BandwidthTotals() (uint64, uint64)

	// ListenAddresses returns a list of addresses at which this network listens.
	ListenAddresses() []ma.Multiaddr

	// InterfaceListenAddresses returns a list of addresses at which this network
	// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
	// use the known local interfaces.
	InterfaceListenAddresses() ([]ma.Multiaddr, error)
}

// Dialer represents a service that can dial out to peers
// (this is usually just a Network, but other services may not need the whole
// stack, and thus it becomes easier to mock)
type Dialer interface {
	// LocalPeer returns the local peer associated with this network
	LocalPeer() peer.Peer

	// DialPeer attempts to establish a connection to a given peer
	DialPeer(context.Context, peer.Peer) error

	// Connectedness returns a state signaling connection capabilities
	Connectedness(peer.Peer) Connectedness
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
