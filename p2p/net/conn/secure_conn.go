package conn

import (
	"errors"
	"net"
	"time"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	ic "github.com/ipfs/go-ipfs/p2p/crypto"
	secio "github.com/ipfs/go-ipfs/p2p/crypto/secio"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
)

// secureConn wraps another Conn object with an encrypted channel.
type secureConn struct {
	insecure Conn          // the wrapped conn
	secure   secio.Session // secure Session
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
		return nil, errors.New("private key is nil")
	}

	// NewSession performs the secure handshake, which takes multiple RTT
	sessgen := secio.SessionGenerator{LocalID: insecure.LocalPeer(), PrivateKey: sk}
	secure, err := sessgen.NewSession(ctx, insecure)
	if err != nil {
		return nil, err
	}

	conn := &secureConn{
		insecure: insecure,
		secure:   secure,
	}
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
	return c.secure.LocalPeer()
}

// RemotePeer is the Peer on the remote side
func (c *secureConn) RemotePeer() peer.ID {
	return c.secure.RemotePeer()
}

// LocalPrivateKey is the public key of the peer on this side
func (c *secureConn) LocalPrivateKey() ic.PrivKey {
	return c.secure.LocalPrivateKey()
}

// RemotePubKey is the public key of the peer on the remote side
func (c *secureConn) RemotePublicKey() ic.PubKey {
	return c.secure.RemotePublicKey()
}

// Read reads data, net.Conn style
func (c *secureConn) Read(buf []byte) (int, error) {
	return c.secure.ReadWriter().Read(buf)
}

// Write writes data, net.Conn style
func (c *secureConn) Write(buf []byte) (int, error) {
	return c.secure.ReadWriter().Write(buf)
}

func (c *secureConn) NextMsgLen() (int, error) {
	return c.secure.ReadWriter().NextMsgLen()
}

// ReadMsg reads data, net.Conn style
func (c *secureConn) ReadMsg() ([]byte, error) {
	return c.secure.ReadWriter().ReadMsg()
}

// WriteMsg writes data, net.Conn style
func (c *secureConn) WriteMsg(buf []byte) error {
	return c.secure.ReadWriter().WriteMsg(buf)
}

// ReleaseMsg releases a buffer
func (c *secureConn) ReleaseMsg(m []byte) {
	c.secure.ReadWriter().ReleaseMsg(m)
}
