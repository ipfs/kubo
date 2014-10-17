package conn

import (
	"errors"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
	msg "github.com/jbenet/go-ipfs/net/message"
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

	// context + cancel
	ctx    context.Context
	cancel context.CancelFunc
	closed chan struct{}

	secure   *spipe.SecurePipe
	insecure *msgioPipe
	msgpipe  *msg.Pipe
}

// Map maps Keys (Peer.IDs) to Connections.
type Map map[u.Key]Conn

// newConn constructs a new connection
func newSingleConn(ctx context.Context, local, remote *peer.Peer,
	peers peer.Peerstore, maconn manet.Conn) (Conn, error) {

	ctx, cancel := context.WithCancel(ctx)

	conn := &singleConn{
		local:    local,
		remote:   remote,
		maconn:   maconn,
		ctx:      ctx,
		cancel:   cancel,
		closed:   make(chan struct{}),
		insecure: newMsgioPipe(10),
		msgpipe:  msg.NewPipe(10),
	}

	log.Info("newSingleConn: %v to %v", local, remote)

	// setup the various io goroutines
	go conn.insecure.outgoing.WriteTo(maconn)
	go conn.insecure.incoming.ReadFrom(maconn, MaxMessageSize)
	go conn.waitToClose(ctx)

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
	var err error
	c.secure, err = spipe.NewSecurePipe(c.ctx, 10, c.local, peers, insecure)
	if err != nil {
		return err
	}

	if c.remote == nil {
		c.remote = c.secure.RemotePeer()

	} else if c.remote != c.secure.RemotePeer() {
		// this panic is here because this would be an insidious programmer error
		// that we need to ensure we catch.
		log.Error("%v != %v", c.remote, c.secure.RemotePeer())
		panic("peers not being constructed correctly.")
	}

	// silly we have to do it this way.
	go c.unwrapOutMsgs()
	go c.wrapInMsgs()

	return nil
}

// unwrapOutMsgs sends just the raw data of a message through secure
func (c *singleConn) unwrapOutMsgs() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case m, more := <-c.msgpipe.Outgoing:
			if !more {
				return
			}

			c.secure.Out <- m.Data()
		}
	}
}

// wrapInMsgs wraps a message
func (c *singleConn) wrapInMsgs() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case d, more := <-c.secure.In:
			if !more {
				return
			}

			c.msgpipe.Incoming <- msg.New(c.remote, d)
		}
	}
}

// waitToClose waits on the given context's Done before closing Conn.
func (c *singleConn) waitToClose(ctx context.Context) {
	select {
	case <-ctx.Done():
	}

	// close underlying connection
	c.maconn.Close()

	// closing channels
	c.insecure.outgoing.Close()
	c.secure.Close()
	close(c.msgpipe.Incoming)
	close(c.closed)
}

// isClosed returns whether this Conn is open or closed.
func (c *singleConn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

// Close closes the connection, and associated channels.
func (c *singleConn) Close() error {
	log.Debug("%s closing Conn with %s", c.local, c.remote)
	if c.isClosed() {
		return fmt.Errorf("connection already closed")
	}

	// cancel context.
	c.cancel()
	return nil
}

// LocalPeer is the Peer on this side
func (c *singleConn) LocalPeer() *peer.Peer {
	return c.local
}

// RemotePeer is the Peer on the remote side
func (c *singleConn) RemotePeer() *peer.Peer {
	return c.remote
}

// MsgIn returns a readable message channel
func (c *singleConn) MsgIn() <-chan msg.NetMessage {
	return c.msgpipe.Incoming
}

// MsgOut returns a writable message channel
func (c *singleConn) MsgOut() chan<- msg.NetMessage {
	return c.msgpipe.Outgoing
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

	// ctx + cancel func
	ctx    context.Context
	cancel context.CancelFunc
	closed chan struct{}
}

// waitToClose is needed to hand
func (l *listener) waitToClose() {
	select {
	case <-l.ctx.Done():
	}

	l.Listener.Close()
	close(l.closed)
}

func (l *listener) isClosed() bool {
	select {
	case <-l.closed:
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
		c, err := newSingleConn(l.ctx, l.local, nil, l.peers, maconn)
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

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors
func (l *listener) Close() error {
	if l.isClosed() {
		return errors.New("listener already closed")
	}

	l.cancel()
	<-l.closed
	return nil
}

// Listen listens on the particular multiaddr, with given peer and peerstore.
func Listen(ctx context.Context, addr ma.Multiaddr, local *peer.Peer, peers peer.Peerstore) (Listener, error) {

	ctx, cancel := context.WithCancel(ctx)

	ml, err := manet.Listen(addr)
	if err != nil {
		return nil, err
	}

	// todo make this a variable
	chansize := 10

	l := &listener{
		ctx:      ctx,
		cancel:   cancel,
		closed:   make(chan struct{}),
		Listener: ml,
		maddr:    addr,
		peers:    peers,
		local:    local,
		conns:    make(chan Conn, chansize),
		chansize: chansize,
	}

	go l.listen()
	go l.waitToClose()

	return l, nil
}
