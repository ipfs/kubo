package conn

import (
	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// Conn is a generic message-based Peer-to-Peer connection.
type Conn interface {

	// LocalPeer is the Peer on this side
	LocalPeer() *peer.Peer

	// RemotePeer is the Peer on the remote side
	RemotePeer() *peer.Peer

	// MsgIn returns a readable message channel
	MsgIn() <-chan msg.NetMessage

	// MsgOut returns a writable message channel
	MsgOut() chan<- msg.NetMessage

	// Close ends the connection
	Close() error
}

// Listener is an object that can accept connections. It matches net.Listener
type Listener interface {

	// Accept waits for and returns the next connection to the listener.
	Accept() <-chan Conn

	// Multiaddr is the identity of the local Peer.
	Multiaddr() ma.Multiaddr

	// LocalPeer is the identity of the local Peer.
	LocalPeer() *peer.Peer

	// Peerstore is the set of peers we know about locally. The Listener needs it
	// because when an incoming connection is identified, we should reuse the
	// same peer objects (otherwise things get inconsistent).
	Peerstore() peer.Peerstore

	// Close closes the listener.
	// Any blocked Accept operations will be unblocked and return errors.
	Close() error
}
