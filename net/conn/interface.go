package conn

import (
	"errors"
	"net"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
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

	// LocalPeer is the Peer on our side of the connection
	LocalPeer() peer.Peer

	// RemoteMultiaddr is the Multiaddr on the remote side
	RemoteMultiaddr() ma.Multiaddr

	// RemotePeer is the Peer on the remote side
	RemotePeer() peer.Peer

	// net.Conn, cause duplicates.
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

// CtxRead is a function that Reads from a connection while respecting a
// Context. Though it cannot cancel the read per-se (as not all Connections
// implement SetTimeout, and a CancelFunc can't be predicted), at least it
// doesn't hang. The Read will eventually return and the goroutine will exit.
func CtxRead(ctx context.Context, c Conn, buf []byte) (n int, err error) {
	done := make(chan struct{})
	go func() {
		n, err = c.Read(buf)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()

	case <-c.Closing():
		return 0, errors.New("remote connection closed")

	case <-done:
		return n, err
	}
}

// CtxReadMsg is a function that Reads from a connection while respecting a
// Context. See CtxRead.
func CtxReadMsg(ctx context.Context, c Conn) (msg []byte, err error) {
	done := make(chan struct{})
	go func() {
		msg, err = c.ReadMsg()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return msg, ctx.Err()

	case <-c.Closing():
		return msg, errors.New("remote connection closed")

	case <-done:
		return msg, err
	}
}

// CtxWrite is a function that Writes to a connection while respecting a
// Context. See CtxRead.
func CtxWrite(ctx context.Context, c Conn, buf []byte) (n int, err error) {
	done := make(chan struct{})
	go func() {
		n, err = c.Read(buf)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()

	case <-c.Closing():
		return 0, errors.New("remote connection closed")

	case <-done:
		return n, err
	}
}

// CtxWriteMsg is a function that Writes to a connection while respecting a
// Context. See CtxRead.
func CtxWriteMsg(ctx context.Context, c Conn, buf []byte) (err error) {
	done := make(chan struct{})
	go func() {
		err = c.WriteMsg(buf)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-c.Closing():
		return errors.New("remote connection closed")

	case <-done:
		return err
	}
}
