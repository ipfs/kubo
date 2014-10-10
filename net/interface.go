package net

import (
	msg "github.com/jbenet/go-ipfs/net/message"
	mux "github.com/jbenet/go-ipfs/net/mux"
	srv "github.com/jbenet/go-ipfs/net/service"
	peer "github.com/jbenet/go-ipfs/peer"
)

// Network is the interface IPFS uses for connecting to the world.
type Network interface {

	// Listen handles incoming connections on given Multiaddr.
	// Listen(*ma.Muliaddr) error
	// TODO: for now, only listen on addrs in local peer when initializing.

	// DialPeer attempts to establish a connection to a given peer
	DialPeer(*peer.Peer) error

	// ClosePeer connection to peer
	ClosePeer(*peer.Peer) error

	// IsConnected returns whether a connection to given peer exists.
	IsConnected(*peer.Peer) (bool, error)

	// GetProtocols returns the protocols registered in the network.
	GetProtocols() *mux.ProtocolMap

	// GetPeerList returns the list of peers currently connected in this network.
	GetPeerList() []*peer.Peer

	// SendMessage sends given Message out
	SendMessage(msg.NetMessage) error

	// Close terminates all network operation
	Close() error
}

// Sender interface for network services.
type Sender srv.Sender

// Handler interface for network services.
type Handler srv.Handler

// Service interface for network resources.
type Service srv.Service
