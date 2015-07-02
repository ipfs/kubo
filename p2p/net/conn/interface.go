package conn

import (
	"io"
	"net"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	ic "github.com/ipfs/go-ipfs/p2p/crypto"
	filter "github.com/ipfs/go-ipfs/p2p/net/filter"
	peer "github.com/ipfs/go-ipfs/p2p/peer"

	msgio "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
)

// Map maps Keys (Peer.IDs) to Connections.
type Map map[key.Key]Conn

type PeerConn interface {
	io.Closer

	// LocalPeer (this side) ID, PrivateKey, and Address
	LocalPeer() peer.ID
	LocalPrivateKey() ic.PrivKey
	LocalMultiaddr() ma.Multiaddr

	// RemotePeer ID, PublicKey, and Address
	RemotePeer() peer.ID
	RemotePublicKey() ic.PubKey
	RemoteMultiaddr() ma.Multiaddr
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
}

// Dialer is an object that can open connections. We could have a "convenience"
// Dial function as before, but it would have many arguments, as dialing is
// no longer simple (need a peerstore, a local peer, a context, a network, etc)
type Dialer struct {

	// Dialer is an optional manet.Dialer to use.
	Dialer manet.Dialer

	// LocalPeer is the identity of the local Peer.
	LocalPeer peer.ID

	// LocalAddrs is a set of local addresses to use.
	LocalAddrs []ma.Multiaddr

	// PrivateKey used to initialize a secure connection.
	// Warning: if PrivateKey is nil, connection will not be secured.
	PrivateKey ic.PrivKey

	// Wrapper to wrap the raw connection (optional)
	Wrapper func(manet.Conn) manet.Conn
}

// Listener is an object that can accept connections. It matches net.Listener
type Listener interface {

	// Accept waits for and returns the next connection to the listener.
	Accept() (net.Conn, error)

	// Addr is the local address
	Addr() net.Addr

	// Multiaddr is the local multiaddr address
	Multiaddr() ma.Multiaddr

	// LocalPeer is the identity of the local Peer.
	LocalPeer() peer.ID

	SetAddrFilters(*filter.Filters)

	// Close closes the listener.
	// Any blocked Accept operations will be unblocked and return errors.
	Close() error
}

// EncryptConnections is a global parameter because it should either be
// enabled or _completely disabled_. I.e. a node should only be able to talk
// to proper (encrypted) networks if it is encrypting all its transports.
// Running a node with disabled transport encryption is useful to debug the
// protocols, achieve implementation interop, or for private networks which
// -- for whatever reason -- _must_ run unencrypted.
var EncryptConnections = true
