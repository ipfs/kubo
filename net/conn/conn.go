package conn

import (
	"fmt"
	"time"

	counting "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/facebookgo/counting"
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	mpool "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio/mpool"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"
)

var log = u.Logger("conn")

const (
	// ChanBuffer is the size of the buffer in the Conn Chan
	ChanBuffer = 10

	// MaxMessageSize is the size of the largest single message. (4MB)
	MaxMessageSize = 1 << 22

	// HandshakeTimeout for when nodes first connect
	HandshakeTimeout = time.Second * 5
)

// ReleaseBuffer puts the given byte array back into the buffer pool,
// first verifying that it is the correct size
func ReleaseBuffer(b []byte) {
	log.Warningf("Releasing buffer! (cap,size = %d, %d)", cap(b), len(b))
	mpool.ByteSlicePool.Put(uint32(cap(b)), b)
}

// msgioPipe is a pipe using msgio channels.
type msgioPipe struct {
	outgoing *msgio.Chan
	incoming *msgio.Chan
}

func newMsgioPipe(size int) *msgioPipe {
	return &msgioPipe{
		outgoing: msgio.NewChan(size),
		incoming: msgio.NewChan(size),
	}
}

// singleConn represents a single connection to another Peer (IPFS Node).
type singleConn struct {
	local        peer.Peer
	remote       peer.Peer
	maconn       manet.Conn
	readCounter  *counting.Reader
	writeCounter *counting.Writer
	msgio        *msgioPipe

	ctxc.ContextCloser
}

// newConn constructs a new connection
func newSingleConn(ctx context.Context, local, remote peer.Peer,
	maconn manet.Conn) (Conn, error) {

	conn := &singleConn{
		local:        local,
		remote:       remote,
		maconn:       maconn,
		readCounter:  counting.NewReader(maconn),
		writeCounter: counting.NewWriter(maconn),
		msgio:        newMsgioPipe(10),
	}

	conn.ContextCloser = ctxc.NewContextCloser(ctx, conn.close)

	log.Info("newSingleConn: %v to %v", local, remote)

	// setup the various io goroutines
	conn.Children().Add(1)
	go func() {
		conn.msgio.outgoing.WriteTo(conn.writeCounter)
		conn.Children().Done()
	}()
	conn.Children().Add(1)
	go func() {
		conn.msgio.incoming.ReadFromWithPool(conn.readCounter, &mpool.ByteSlicePool)
		conn.Children().Done()
	}()

	// version handshake
	ctxT, _ := context.WithTimeout(ctx, HandshakeTimeout)
	if err := Handshake1(ctxT, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("Handshake1 failed: %s", err)
	}

	return conn, nil
}

// close is the internal close function, called by ContextCloser.Close
func (c *singleConn) close() error {
	log.Debugf("%s closing Conn with %s", c.local, c.remote)

	// close underlying connection
	err := c.maconn.Close()
	c.msgio.outgoing.Close()
	return err
}

func (c *singleConn) GetError() error {
	select {
	case err := <-c.msgio.incoming.ErrChan:
		return err
	case err := <-c.msgio.outgoing.ErrChan:
		return err
	default:
		return nil
	}
}

// ID is an identifier unique to this connection.
func (c *singleConn) ID() string {
	return ID(c)
}

func (c *singleConn) String() string {
	return String(c, "singleConn")
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

// In returns a readable message channel
func (c *singleConn) In() <-chan []byte {
	return c.msgio.incoming.MsgChan
}

// Out returns a writable message channel
func (c *singleConn) Out() chan<- []byte {
	return c.msgio.outgoing.MsgChan
}

// BytesRead returns the total number of bytes received from the peer
func (c *singleConn) BytesRead() int {
	return c.readCounter.Count()
}

// BytesWritten returns the total number of bytes sent to the peer
func (c *singleConn) BytesWritten() int {
	return c.writeCounter.Count()
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
	return fmt.Sprintf("%s (%s) <-- %s --> (%s) %s",
		c.LocalPeer(), c.LocalMultiaddr(), typ, c.RemoteMultiaddr(), c.RemotePeer())
}
