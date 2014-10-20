package conn

import (
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// Map maps Keys (Peer.IDs) to Connections.
type Map map[u.Key]Conn

// Conn is a generic message-based Peer-to-Peer connection.
type Conn interface {
	// implement ContextCloser too!
	ctxc.ContextCloser

	// ID is an identifier unique to this connection.
	ID() string

	// LocalMultiaddr is the Multiaddr on this side
	LocalMultiaddr() ma.Multiaddr

	// LocalPeer is the Peer on this side
	LocalPeer() peer.Peer

	// RemoteMultiaddr is the Multiaddr on the remote side
	RemoteMultiaddr() ma.Multiaddr

	// RemotePeer is the Peer on the remote side
	RemotePeer() peer.Peer

	// In returns a readable message channel
	In() <-chan []byte

	// Out returns a writable message channel
	Out() chan<- []byte

	// Close ends the connection
	// Close() error  -- already in ContextCloser
}

// Dialer is an object that can open connections. We could have a "convenience"
// Dial function as before, but it would have many arguments, as dialing is
// no longer simple (need a peerstore, a local peer, a context, a network, etc)
type Dialer struct {

	// LocalPeer is the identity of the local Peer.
	LocalPeer peer.Peer

	// Peerstore is the set of peers we know about locally. The Dialer needs it
	// because when an incoming connection is identified, we should reuse the
	// same peer objects (otherwise things get inconsistent).
	Peerstore peer.Peerstore
}

// Listener is an object that can accept connections. It matches net.Listener
type Listener interface {

	// Accept waits for and returns the next connection to the listener.
	Accept() <-chan Conn

	// Multiaddr is the identity of the local Peer.
	Multiaddr() ma.Multiaddr

	// LocalPeer is the identity of the local Peer.
	LocalPeer() peer.Peer

	// Peerstore is the set of peers we know about locally. The Listener needs it
	// because when an incoming connection is identified, we should reuse the
	// same peer objects (otherwise things get inconsistent).
	Peerstore() peer.Peerstore

	// Close closes the listener.
	// Any blocked Accept operations will be unblocked and return errors.
	Close() error
}
