package conn

import (
	"io"
	"net"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// Map maps Keys (Peer.IDs) to Connections.
type Map map[u.Key]Conn

type PeerConn interface {
	// LocalMultiaddr is the Multiaddr on this side
	LocalMultiaddr() ma.Multiaddr

	// LocalPeer is the Peer on our side of the connection
	LocalPeer() peer.Peer

	// RemoteMultiaddr is the Multiaddr on the remote side
	RemoteMultiaddr() ma.Multiaddr

	// RemotePeer is the Peer on the remote side
	RemotePeer() peer.Peer
}

// Conn is a generic message-based Peer-to-Peer connection.
type Conn interface {
	PeerConn

	// ID is an identifier unique to this connection.
	ID() string

	// can't just say "net.Conn" cause we have duplicate methods.
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error

	msgio.Reader
	msgio.Writer
	io.Closer
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

	// WithoutSecureTransport determines whether to initialize an insecure connection.
	// Phrased negatively so default is Secure, and verbosely to be very clear.
	WithoutSecureTransport bool
}

// Listener is an object that can accept connections. It matches net.Listener
type Listener interface {

	// Accept waits for and returns the next connection to the listener.
	Accept() (net.Conn, error)

	// {Set}WithoutSecureTransport decides whether to start insecure connections.
	// Phrased negatively so default is Secure, and verbosely to be very clear.
	WithoutSecureTransport() bool
	SetWithoutSecureTransport(bool)

	// Addr is the local address
	Addr() net.Addr

	// Multiaddr is the local multiaddr address
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
