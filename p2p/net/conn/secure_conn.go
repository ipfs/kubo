package conn

import (
	"net"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	secio "github.com/jbenet/go-ipfs/p2p/crypto/secio"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

// secureConn wraps another Conn object with an encrypted channel.
type secureConn struct {

	// the wrapped conn
	insecure Conn

	// secure io (wrapping insecure)
	secure msgio.ReadWriteCloser

	// secure Session
	session secio.Session
}

// newConn constructs a new connection
func newSecureConn(ctx context.Context, sk ic.PrivKey, insecure Conn) (Conn, error) {

	if insecure == nil {
		return nil, errors.New("insecure is nil")
	}
	if insecure.LocalPeer() == "" {
		return nil, errors.New("insecure.LocalPeer() is nil")
	}
	if sk == nil {
		panic("way")
		return nil, errors.New("private key is nil")
	}

	// NewSession performs the secure handshake, which takes multiple RTT
	sessgen := secio.SessionGenerator{LocalID: insecure.LocalPeer(), PrivateKey: sk}
	session, err := sessgen.NewSession(ctx, insecure)
	if err != nil {
		return nil, err
	}

	conn := &secureConn{
		insecure: insecure,
		session:  session,
		secure:   session.ReadWriter(),
	}
	log.Debugf("newSecureConn: %v to %v handshake success!", conn.LocalPeer(), conn.RemotePeer())
	return conn, nil
}

func (c *secureConn) Close() error {
	return c.secure.Close()
}

// ID is an identifier unique to this connection.
func (c *secureConn) ID() string {
	return ID(c)
}

func (c *secureConn) String() string {
	return String(c, "secureConn")
}

func (c *secureConn) LocalAddr() net.Addr {
	return c.insecure.LocalAddr()
}

func (c *secureConn) RemoteAddr() net.Addr {
	return c.insecure.RemoteAddr()
}

func (c *secureConn) SetDeadline(t time.Time) error {
	return c.insecure.SetDeadline(t)
}

func (c *secureConn) SetReadDeadline(t time.Time) error {
	return c.insecure.SetReadDeadline(t)
}

func (c *secureConn) SetWriteDeadline(t time.Time) error {
	return c.insecure.SetWriteDeadline(t)
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
func (c *secureConn) LocalPeer() peer.ID {
	return c.session.LocalPeer()
}

// RemotePeer is the Peer on the remote side
func (c *secureConn) RemotePeer() peer.ID {
	return c.session.RemotePeer()
}

// LocalPrivateKey is the public key of the peer on this side
func (c *secureConn) LocalPrivateKey() ic.PrivKey {
	return c.session.LocalPrivateKey()
}

// RemotePubKey is the public key of the peer on the remote side
func (c *secureConn) RemotePublicKey() ic.PubKey {
	return c.session.RemotePublicKey()
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
