package conn

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
	peer "github.com/jbenet/go-ipfs/peer"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"
	"github.com/jbenet/go-ipfs/util/pipes"
)

// secureConn wraps another Conn object with an encrypted channel.
type secureConn struct {

	// the wrapped conn
	insecure Conn

	// secure pipe, wrapping insecure
	secure *spipe.SecurePipe

	ctxc.ContextCloser
}

// newConn constructs a new connection
func newSecureConn(ctx context.Context, insecure Conn, peers peer.Peerstore) (Conn, error) {

	conn := &secureConn{
		insecure: insecure,
	}
	conn.ContextCloser = ctxc.NewContextCloser(ctx, conn.close)

	log.Debugf("newSecureConn: %v to %v", insecure.LocalPeer(), insecure.RemotePeer())
	// perform secure handshake before returning this connection.
	if err := conn.secureHandshake(peers); err != nil {
		conn.Close()
		return nil, err
	}
	log.Debugf("newSecureConn: %v to %v handshake success!", insecure.LocalPeer(), insecure.RemotePeer())

	return conn, nil
}

// secureHandshake performs the spipe secure handshake.
func (c *secureConn) secureHandshake(peers peer.Peerstore) error {
	if c.secure != nil {
		return errors.New("Conn is already secured or being secured.")
	}

	// ok to panic here if this type assertion fails. Interface hack.
	// when we support wrapping other Conns, we'll need to change
	// spipe to do something else.
	insecureSC := c.insecure.(*singleConn)

	// setup a Duplex pipe for spipe
	insecureD := pipes.Duplex{
		In:  insecureSC.msgio.incoming.MsgChan,
		Out: insecureSC.msgio.outgoing.MsgChan,
	}

	// spipe performs the secure handshake, which takes multiple RTT
	sp, err := spipe.NewSecurePipe(c.Context(), 10, c.LocalPeer(), peers, insecureD)
	if err != nil {
		return err
	}

	// assign it into the conn object
	c.secure = sp

	// if we do not know RemotePeer, get it from secure chan (who identifies it)
	if insecureSC.remote == nil {
		insecureSC.remote = c.secure.RemotePeer()

	} else if insecureSC.remote != c.secure.RemotePeer() {
		// this panic is here because this would be an insidious programmer error
		// that we need to ensure we catch.
		// update: this actually might happen under normal operation-- should
		// perhaps return an error. TBD.

		log.Errorf("secureConn peer mismatch. %v != %v", insecureSC.remote, c.secure.RemotePeer())
		log.Errorf("insecureSC.remote: %s %#v", insecureSC.remote, insecureSC.remote)
		log.Errorf("c.secure.LocalPeer: %s %#v", c.secure.RemotePeer(), c.secure.RemotePeer())
		panic("secureConn peer mismatch. consructed incorrectly?")
	}

	return nil
}

// close is called by ContextCloser
func (c *secureConn) close() error {
	err := c.insecure.Close()
	if c.secure != nil { // may never have gotten here.
		err = c.secure.Close()
	}
	return err
}

// ID is an identifier unique to this connection.
func (c *secureConn) ID() string {
	return ID(c)
}

func (c *secureConn) String() string {
	return String(c, "secureConn")
}

// LocalMultiaddr is the Multiaddr on this side
func (c *secureConn) LocalMultiaddr() ma.Multiaddr {
	return c.insecure.LocalMultiaddr()
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *secureConn) RemoteMultiaddr() ma.Multiaddr {
	return c.insecure.RemoteMultiaddr()
}

// LocalPeer is the Peer on this side
func (c *secureConn) LocalPeer() peer.Peer {
	return c.insecure.LocalPeer()
}

// RemotePeer is the Peer on the remote side
func (c *secureConn) RemotePeer() peer.Peer {
	return c.insecure.RemotePeer()
}

// In returns a readable message channel
func (c *secureConn) In() <-chan []byte {
	return c.secure.In
}

// Out returns a writable message channel
func (c *secureConn) Out() chan<- []byte {
	return c.secure.Out
}

func (c *secureConn) GetError() error {
	return c.insecure.GetError()
}
