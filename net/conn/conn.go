package conn

import (
	"fmt"
	"net"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	mpool "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio/mpool"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
)

var log = eventlog.Logger("conn")

const (
	// MaxMessageSize is the size of the largest single message. (4MB)
	MaxMessageSize = 1 << 22
)

// ReleaseBuffer puts the given byte array back into the buffer pool,
// first verifying that it is the correct size
func ReleaseBuffer(b []byte) {
	log.Debugf("Releasing buffer! (cap,size = %d, %d)", cap(b), len(b))
	mpool.ByteSlicePool.Put(uint32(cap(b)), b)
}

// singleConn represents a single connection to another Peer (IPFS Node).
type singleConn struct {
	local  peer.Peer
	remote peer.Peer
	maconn manet.Conn
	msgrw  msgio.ReadWriteCloser
}

// newConn constructs a new connection
func newSingleConn(ctx context.Context, local, remote peer.Peer, maconn manet.Conn) (Conn, error) {

	conn := &singleConn{
		local:  local,
		remote: remote,
		maconn: maconn,
		msgrw:  msgio.NewReadWriter(maconn),
	}
	log.Debugf("newSingleConn %p: %v to %v", conn, local, remote)

	// version handshake
	if err := Handshake1(ctx, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Handshake1 failed: %s", err)
	}

	log.Debugf("newSingleConn %p: %v to %v finished", conn, local, remote)
	return conn, nil
}

// close is the internal close function, called by ContextCloser.Close
func (c *singleConn) Close() error {
	log.Debugf("%s closing Conn with %s", c.local, c.remote)
	// close underlying connection
	return c.msgrw.Close()
}

// ID is an identifier unique to this connection.
func (c *singleConn) ID() string {
	return ID(c)
}

func (c *singleConn) String() string {
	return String(c, "singleConn")
}

func (c *singleConn) LocalAddr() net.Addr {
	return c.maconn.LocalAddr()
}

func (c *singleConn) RemoteAddr() net.Addr {
	return c.maconn.RemoteAddr()
}

func (c *singleConn) SetDeadline(t time.Time) error {
	return c.maconn.SetDeadline(t)
}
func (c *singleConn) SetReadDeadline(t time.Time) error {
	return c.maconn.SetReadDeadline(t)
}

func (c *singleConn) SetWriteDeadline(t time.Time) error {
	return c.maconn.SetWriteDeadline(t)
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
func (c *singleConn) LocalPeer() peer.Peer {
	return c.local
}

// RemotePeer is the Peer on the remote side
func (c *singleConn) RemotePeer() peer.Peer {
	return c.remote
}

// Read reads data, net.Conn style
func (c *singleConn) Read(buf []byte) (int, error) {
	return c.msgrw.Read(buf)
}

// Write writes data, net.Conn style
func (c *singleConn) Write(buf []byte) (int, error) {
	return c.msgrw.Write(buf)
}

func (c *singleConn) NextMsgLen() (int, error) {
	return c.msgrw.NextMsgLen()
}

// ReadMsg reads data, net.Conn style
func (c *singleConn) ReadMsg() ([]byte, error) {
	return c.msgrw.ReadMsg()
}

// WriteMsg writes data, net.Conn style
func (c *singleConn) WriteMsg(buf []byte) error {
	return c.msgrw.WriteMsg(buf)
}

// ReleaseMsg releases a buffer
func (c *singleConn) ReleaseMsg(m []byte) {
	c.msgrw.ReleaseMsg(m)
}

// ID returns the ID of a given Conn.
func ID(c Conn) string {
	l := fmt.Sprintf("%s/%s", c.LocalMultiaddr(), c.LocalPeer().ID())
	r := fmt.Sprintf("%s/%s", c.RemoteMultiaddr(), c.RemotePeer().ID())
	lh := u.Hash([]byte(l))
	rh := u.Hash([]byte(r))
	ch := u.XOR(lh, rh)
	return u.Key(ch).Pretty()
}

// String returns the user-friendly String representation of a conn
func String(c Conn, typ string) string {
	return fmt.Sprintf("%s (%s) <-- %s %p --> (%s) %s",
		c.LocalPeer(), c.LocalMultiaddr(), typ, c, c.RemoteMultiaddr(), c.RemotePeer())
}
