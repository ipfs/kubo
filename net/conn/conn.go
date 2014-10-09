package conn

import (
	"fmt"
	"net"

	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// ChanBuffer is the size of the buffer in the Conn Chan
const ChanBuffer = 10

// 1 MB
const MaxMessageSize = 1 << 20

// Conn represents a connection to another Peer (IPFS Node).
type Conn struct {
	Peer *peer.Peer
	Addr ma.Multiaddr
	Conn net.Conn

	Closed   chan bool
	Outgoing *msgio.Chan
	Incoming *msgio.Chan
	Secure   *spipe.SecurePipe
}

// Map maps Keys (Peer.IDs) to Connections.
type Map map[u.Key]*Conn

// NewConn constructs a new connection
func NewConn(peer *peer.Peer, addr ma.Multiaddr, nconn net.Conn) (*Conn, error) {
	conn := &Conn{
		Peer: peer,
		Addr: addr,
		Conn: nconn,
	}

	if err := conn.newChans(); err != nil {
		return nil, err
	}

	return conn, nil
}

// Dial connects to a particular peer, over a given network
// Example: Dial("udp", peer)
func Dial(network string, peer *peer.Peer) (*Conn, error) {
	addr := peer.NetAddress(network)
	if addr == nil {
		return nil, fmt.Errorf("No address for network %s", network)
	}

	network, host, err := ma.DialArgs(addr)
	if err != nil {
		return nil, err
	}

	nconn, err := net.Dial(network, host)
	if err != nil {
		return nil, err
	}

	return NewConn(peer, addr, nconn)
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
	u.DOut("Closing Conn.\n")
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

// NetConnMultiaddr returns the net.Conn's address, recast as a multiaddr.
// (consider moving this directly into the multiaddr package)
func NetConnMultiaddr(nconn net.Conn) (ma.Multiaddr, error) {
	return ma.FromNetAddr(nconn.RemoteAddr())
}
