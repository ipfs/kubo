package conn

import (
	"fmt"

	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
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

// Conn represents a connection to another Peer (IPFS Node).
type Conn struct {
	Local  *peer.Peer
	Remote *peer.Peer
	Conn   manet.Conn

	Closed   chan bool
	Outgoing *msgio.Chan
	Incoming *msgio.Chan
	Secure   *spipe.SecurePipe
}

// Map maps Keys (Peer.IDs) to Connections.
type Map map[u.Key]*Conn

// NewConn constructs a new connection
func NewConn(local, remote *peer.Peer, mconn manet.Conn) (*Conn, error) {
	conn := &Conn{
		Local:  local,
		Remote: remote,
		Conn:   mconn,
	}

	if err := conn.newChans(); err != nil {
		return nil, err
	}

	return conn, nil
}

// Dial connects to a particular peer, over a given network
// Example: Dial("udp", peer)
func Dial(network string, local, remote *peer.Peer) (*Conn, error) {
	laddr := local.NetAddress(network)
	if laddr == nil {
		return nil, fmt.Errorf("No local address for network %s", network)
	}

	raddr := remote.NetAddress(network)
	if raddr == nil {
		return nil, fmt.Errorf("No remote address for network %s", network)
	}

	// TODO: try to get reusing addr/ports to work.
	// dialer := manet.Dialer{LocalAddr: laddr}
	dialer := manet.Dialer{}

	log.Info("%s %s dialing %s %s", local, laddr, remote, raddr)
	nconn, err := dialer.Dial(raddr)
	if err != nil {
		return nil, err
	}

	return NewConn(local, remote, nconn)
}

// Construct new channels for given Conn.
func (c *Conn) newChans() error {
	if c.Outgoing != nil || c.Incoming != nil {
		return fmt.Errorf("Conn already initialized")
	}

	c.Outgoing = msgio.NewChan(10)
	c.Incoming = msgio.NewChan(10)
	c.Closed = make(chan bool, 1)

	go c.Outgoing.WriteTo(c.Conn)
	go c.Incoming.ReadFrom(c.Conn, MaxMessageSize)

	return nil
}

// Close closes the connection, and associated channels.
func (c *Conn) Close() error {
	log.Debug("%s closing Conn with %s", c.Local, c.Remote)
	if c.Conn == nil {
		return fmt.Errorf("Already closed") // already closed
	}

	// closing net connection
	err := c.Conn.Close()
	c.Conn = nil
	// closing channels
	c.Incoming.Close()
	c.Outgoing.Close()
	c.Closed <- true
	return err
}
