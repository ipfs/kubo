package conn

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	secio "github.com/jbenet/go-ipfs/crypto/secio"
	peer "github.com/jbenet/go-ipfs/peer"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"
)

// secureConn wraps another Conn object with an encrypted channel.
type secureConn struct {

	// the wrapped conn
	insecure Conn

	// secure io (wrapping insecure)
	secure msgio.ReadWriteCloser

	// secure Session
	session secio.Session

	ctxc.ContextCloser
}

// newConn constructs a new connection
func newSecureConn(ctx context.Context, insecure Conn, peers peer.Peerstore) (Conn, error) {

	// NewSession performs the secure handshake, which takes multiple RTT
	sessgen := secio.SessionGenerator{Local: insecure.LocalPeer(), Peerstore: peers}
	session, err := sessgen.NewSession(ctx, insecure)
	if err != nil {
		return nil, err
	}

	conn := &secureConn{
		insecure: insecure,
		session:  session,
		secure:   session.ReadWriter(),
	}
	conn.ContextCloser = ctxc.NewContextCloser(ctx, conn.close)
	log.Debugf("newSecureConn: %v to %v handshake success!", conn.LocalPeer(), conn.RemotePeer())
	return conn, nil
}

// close is called by ContextCloser
func (c *secureConn) close() error {
	if err := c.secure.Close(); err != nil {
		c.insecure.Close()
		return err
	}
	return c.insecure.Close()
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
	return c.session.LocalPeer()
}

// RemotePeer is the Peer on the remote side
func (c *secureConn) RemotePeer() peer.Peer {
	return c.session.RemotePeer()
}

// Read reads data, net.Conn style
func (c *secureConn) Read(buf []byte) (int, error) {
	return c.secure.Read(buf)
}

// Write writes data, net.Conn style
func (c *secureConn) Write(buf []byte) (int, error) {
	return c.secure.Write(buf)
}

func (c *secureConn) NextMsgLen() (int, error) {
	return c.secure.NextMsgLen()
}

// ReadMsg reads data, net.Conn style
func (c *secureConn) ReadMsg() ([]byte, error) {
	return c.secure.ReadMsg()
}

// WriteMsg writes data, net.Conn style
func (c *secureConn) WriteMsg(buf []byte) error {
	return c.secure.WriteMsg(buf)
}

// ReleaseMsg releases a buffer
func (c *secureConn) ReleaseMsg(m []byte) {
	c.secure.ReleaseMsg(m)
}
