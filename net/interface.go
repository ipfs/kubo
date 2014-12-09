package net

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	conn "github.com/jbenet/go-ipfs/net/conn"
	msg "github.com/jbenet/go-ipfs/net/message"
	mux "github.com/jbenet/go-ipfs/net/mux"
	srv "github.com/jbenet/go-ipfs/net/service"
	peer "github.com/jbenet/go-ipfs/peer"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// Network is the interface IPFS uses for connecting to the world.
type Network interface {
	ctxc.ContextCloser

	// Listen handles incoming connections on given Multiaddr.
	// Listen(*ma.Muliaddr) error
	// TODO: for now, only listen on addrs in local peer when initializing.

	// LocalPeer returns the local peer associated with this network
	LocalPeer() peer.Peer

	// DialPeer attempts to establish a connection to a given peer
	DialPeer(context.Context, peer.Peer) error

	// ClosePeer connection to peer
	ClosePeer(peer.Peer) error

	// Connectedness returns a state signaling connection capabilities
	Connectedness(peer.Peer) Connectedness

	// GetProtocols returns the protocols registered in the network.
	GetProtocols() *mux.ProtocolMap

	// GetPeerList returns the list of peers currently connected in this network.
	GetPeerList() []peer.Peer

	// GetConnections returns the list of connections currently open in this network.
	GetConnections() []conn.Conn

	// GetBandwidthTotals returns the total number of bytes passed through
	// the network since it was instantiated
	GetBandwidthTotals() (uint64, uint64)

	// GetMessageCounts returns the total number of messages passed through
	// the network since it was instantiated
	GetMessageCounts() (uint64, uint64)

	// SendMessage sends given Message out
	SendMessage(msg.NetMessage) error

	// ListenAddresses returns a list of addresses at which this network listens.
	ListenAddresses() []ma.Multiaddr

	// InterfaceListenAddresses returns a list of addresses at which this network
	// listens. It expands "any interface" addresses (/ip4/0.0.0.0, /ip6/::) to
	// use the known local interfaces.
	InterfaceListenAddresses() ([]ma.Multiaddr, error)
}

// Sender interface for network services.
type Sender srv.Sender

// Handler interface for network services.
type Handler srv.Handler

// Service interface for network resources.
type Service srv.Service

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
