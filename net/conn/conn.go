package conn

import (
	"errors"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("conn")

// ChanBuffer is the size of the buffer in the Conn Chan
const ChanBuffer = 10

// 1 MB
const MaxMessageSize = 1 << 20

// msgioPipe is a pipe using msgio channels.
type msgioPipe struct {
	outgoing *msgio.Chan
	incoming *msgio.Chan
}

func newMsgioPipe(size int) *msgioPipe {
	return &msgioPipe{
		outgoing: msgio.NewChan(10),
		incoming: msgio.NewChan(10),
	}
}

// singleConn represents a single connection to another Peer (IPFS Node).
type singleConn struct {
	local  *peer.Peer
	remote *peer.Peer
	maconn manet.Conn

	secure   *spipe.SecurePipe
	insecure *msgioPipe

	ContextCloser
}

// newConn constructs a new connection
func newSingleConn(ctx context.Context, local, remote *peer.Peer,
	peers peer.Peerstore, maconn manet.Conn) (Conn, error) {

	conn := &singleConn{
		local:    local,
		remote:   remote,
		maconn:   maconn,
		insecure: newMsgioPipe(10),
	}

	conn.ContextCloser = NewContextCloser(ctx, conn.close)

	log.Info("newSingleConn: %v to %v", local, remote)

	// setup the various io goroutines
	go conn.insecure.outgoing.WriteTo(maconn)
	go conn.insecure.incoming.ReadFrom(maconn, MaxMessageSize)

	// perform secure handshake before returning this connection.
	if err := conn.secureHandshake(peers); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// secureHandshake performs the spipe secure handshake.
func (c *singleConn) secureHandshake(peers peer.Peerstore) error {
	if c.secure != nil {
		return errors.New("Conn is already secured or being secured.")
	}

	// setup a Duplex pipe for spipe
	insecure := spipe.Duplex{
		In:  c.insecure.incoming.MsgChan,
		Out: c.insecure.outgoing.MsgChan,
	}

	// spipe performs the secure handshake, which takes multiple RTT
	sp, err := spipe.NewSecurePipe(c.Context(), 10, c.local, peers, insecure)
	if err != nil {
		return err
	}

	// assign it into the conn object
	c.secure = sp

	if c.remote == nil {
		c.remote = c.secure.RemotePeer()

	} else if c.remote != c.secure.RemotePeer() {
		// this panic is here because this would be an insidious programmer error
		// that we need to ensure we catch.
		log.Error("%v != %v", c.remote, c.secure.RemotePeer())
		panic("peers not being constructed correctly.")
	}

	return nil
}

// close is the internal close function, called by ContextCloser.Close
func (c *singleConn) close() error {
	log.Debug("%s closing Conn with %s", c.local, c.remote)

	// close underlying connection
	err := c.maconn.Close()

	// closing channels
	c.insecure.outgoing.Close()
	if c.secure != nil { // may never have gotten here.
		c.secure.Close()
	}

	return err
}

// LocalPeer is the Peer on this side
func (c *singleConn) LocalPeer() *peer.Peer {
	return c.local
}

// RemotePeer is the Peer on the remote side
func (c *singleConn) RemotePeer() *peer.Peer {
	return c.remote
}

// In returns a readable message channel
func (c *singleConn) In() <-chan []byte {
	return c.secure.In
}

// Out returns a writable message channel
func (c *singleConn) Out() chan<- []byte {
	return c.secure.Out
}

// Dialer is an object that can open connections. We could have a "convenience"
// Dial function as before, but it would have many arguments, as dialing is
// no longer simple (need a peerstore, a local peer, a context, a network, etc)
type Dialer struct {

	// LocalPeer is the identity of the local Peer.
	LocalPeer *peer.Peer

	// Peerstore is the set of peers we know about locally. The Dialer needs it
	// because when an incoming connection is identified, we should reuse the
	// same peer objects (otherwise things get inconsistent).
	Peerstore peer.Peerstore
}

// Dial connects to a particular peer, over a given network
// Example: d.Dial(ctx, "udp", peer)
func (d *Dialer) Dial(ctx context.Context, network string, remote *peer.Peer) (Conn, error) {
	laddr := d.LocalPeer.NetAddress(network)
	if laddr == nil {
		return nil, fmt.Errorf("No local address for network %s", network)
	}

	raddr := remote.NetAddress(network)
	if raddr == nil {
		return nil, fmt.Errorf("No remote address for network %s", network)
	}

	// TODO: try to get reusing addr/ports to work.
	// madialer := manet.Dialer{LocalAddr: laddr}
	madialer := manet.Dialer{}

	log.Info("%s dialing %s %s", d.LocalPeer, remote, raddr)
	maconn, err := madialer.Dial(raddr)
	if err != nil {
		return nil, err
	}

	if err := d.Peerstore.Put(remote); err != nil {
		log.Error("Error putting peer into peerstore: %s", remote)
	}

	return newSingleConn(ctx, d.LocalPeer, remote, d.Peerstore, maconn)
}

// listener is an object that can accept connections. It implements Listener
type listener struct {
	manet.Listener

	// chansize is the size of the internal channels for concurrency
	chansize int

	// channel of incoming conections
	conns chan Conn

	// Local multiaddr to listen on
	maddr ma.Multiaddr

	// LocalPeer is the identity of the local Peer.
	local *peer.Peer

	// Peerstore is the set of peers we know about locally
	peers peer.Peerstore

	// embedded ContextCloser
	ContextCloser
}

// disambiguate
func (l *listener) Close() error {
	return l.ContextCloser.Close()
}

// close called by ContextCloser.Close
func (l *listener) close() error {
	log.Info("listener closing: %s %s", l.local, l.maddr)
	return l.Listener.Close()
}

func (l *listener) isClosed() bool {
	select {
	case <-l.Done():
		return true
	default:
		return false
	}
}

func (l *listener) listen() {

	// handle at most chansize concurrent handshakes
	sem := make(chan struct{}, l.chansize)

	// handle is a goroutine work function that handles the handshake.
	// it's here only so that accepting new connections can happen quickly.
	handle := func(maconn manet.Conn) {
		c, err := newSingleConn(l.Context(), l.local, nil, l.peers, maconn)
		if err != nil {
			log.Error("Error accepting connection: %v", err)
		} else {
			l.conns <- c
		}
		<-sem // release
	}

	for {
		maconn, err := l.Listener.Accept()
		if err != nil {

			// if cancel is nil we're closed.
			if l.isClosed() {
				return // done.
			}

			log.Error("Failed to accept connection: %v", err)
			continue
		}

		sem <- struct{}{} // acquire
		go handle(maconn)
	}
}

// Accept waits for and returns the next connection to the listener.
// Note that unfortunately this
func (l *listener) Accept() <-chan Conn {
	return l.conns
}

// Multiaddr is the identity of the local Peer.
func (l *listener) Multiaddr() ma.Multiaddr {
	return l.maddr
}

// LocalPeer is the identity of the local Peer.
func (l *listener) LocalPeer() *peer.Peer {
	return l.local
}

// Peerstore is the set of peers we know about locally. The Listener needs it
// because when an incoming connection is identified, we should reuse the
// same peer objects (otherwise things get inconsistent).
func (l *listener) Peerstore() peer.Peerstore {
	return l.peers
}

// Listen listens on the particular multiaddr, with given peer and peerstore.
func Listen(ctx context.Context, addr ma.Multiaddr, local *peer.Peer, peers peer.Peerstore) (Listener, error) {

	ml, err := manet.Listen(addr)
	if err != nil {
		return nil, err
	}

	// todo make this a variable
	chansize := 10

	l := &listener{
		Listener: ml,
		maddr:    addr,
		peers:    peers,
		local:    local,
		conns:    make(chan Conn, chansize),
		chansize: chansize,
	}

	l.ContextCloser = NewContextCloser(ctx, l.close)

	go l.listen()

	return l, nil
}
