package conn

import (
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("conn")

const (
	// ChanBuffer is the size of the buffer in the Conn Chan
	ChanBuffer = 10

	// MaxMessageSize is the size of the largest single message
	MaxMessageSize = 1 << 20 // 1 MB

	// HandshakeTimeout for when nodes first connect
	HandshakeTimeout = time.Second * 5
)

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
	msgio  *msgioPipe

	ContextCloser
}

// newConn constructs a new connection
func newSingleConn(ctx context.Context, local, remote *peer.Peer,
	maconn manet.Conn) (Conn, error) {

	conn := &singleConn{
		local:  local,
		remote: remote,
		maconn: maconn,
		msgio:  newMsgioPipe(10),
	}

	conn.ContextCloser = NewContextCloser(ctx, conn.close)

	log.Info("newSingleConn: %v to %v", local, remote)

	// setup the various io goroutines
	go func() {
		conn.Children().Add(1)
		conn.msgio.outgoing.WriteTo(maconn)
		conn.Children().Done()
	}()
	go func() {
		conn.Children().Add(1)
		conn.msgio.incoming.ReadFrom(maconn, MaxMessageSize)
		conn.Children().Done()
	}()

	// version handshake
	ctxT, _ := context.WithTimeout(ctx, HandshakeTimeout)
	if err := VersionHandshake(ctxT, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Version handshake: %s", err)
	}

	return conn, nil
}

// close is the internal close function, called by ContextCloser.Close
func (c *singleConn) close() error {
	log.Debug("%s closing Conn with %s", c.local, c.remote)

	// close underlying connection
	err := c.maconn.Close()
	c.msgio.outgoing.Close()
	return err
}

// ID is an identifier unique to this connection.
func (c *singleConn) ID() string {
	return ID(c)
}

// LocalMultiaddr is the Multiaddr on this side
func (c *singleConn) LocalMultiaddr() ma.Multiaddr {
	return c.maconn.LocalMultiaddr()
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *singleConn) RemoteMultiaddr() ma.Multiaddr {
	return c.maconn.RemoteMultiaddr()
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
	return c.msgio.incoming.MsgChan
}

// Out returns a writable message channel
func (c *singleConn) Out() chan<- []byte {
	return c.msgio.outgoing.MsgChan
}

// ID returns the
func ID(c Conn) string {
	l := fmt.Sprintf("%s/%s", c.LocalMultiaddr(), c.LocalPeer().ID)
	r := fmt.Sprintf("%s/%s", c.RemoteMultiaddr(), c.RemotePeer().ID)
	lh := u.Hash([]byte(l))
	rh := u.Hash([]byte(r))
	ch := u.XOR(lh, rh)
	return u.Key(ch).Pretty()
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

	c, err := newSingleConn(ctx, d.LocalPeer, remote, maconn)
	if err != nil {
		return nil, err
	}

	return newSecureConn(ctx, c, d.Peerstore)
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

func (l *listener) listen() {
	l.Children().Add(1)
	defer l.Children().Done()

	// handle at most chansize concurrent handshakes
	sem := make(chan struct{}, l.chansize)

	// handle is a goroutine work function that handles the handshake.
	// it's here only so that accepting new connections can happen quickly.
	handle := func(maconn manet.Conn) {
		defer func() { <-sem }() // release

		c, err := newSingleConn(l.Context(), l.local, nil, maconn)
		if err != nil {
			log.Error("Error accepting connection: %v", err)
			return
		}

		sc, err := newSecureConn(l.Context(), c, l.peers)
		if err != nil {
			log.Error("Error securing connection: %v", err)
			return
		}

		l.conns <- sc
	}

	for {
		maconn, err := l.Listener.Accept()
		if err != nil {

			// if closing, we should exit.
			select {
			case <-l.Closing():
				return // done.
			default:
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
