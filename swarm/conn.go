package swarm

import (
	"fmt"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	msgio "github.com/jbenet/go-msgio"
	ma "github.com/jbenet/go-multiaddr"
	"net"
)

// ChanBuffer is the size of the buffer in the Conn Chan
const ChanBuffer = 10

// Conn represents a connection to another Peer (IPFS Node).
type Conn struct {
	Peer *peer.Peer
	Addr *ma.Multiaddr
	Conn net.Conn

	Closed   chan bool
	Outgoing *msgio.Chan
	Incoming *msgio.Chan
}

// ConnMap maps Keys (PeerIds) to Connections.
type ConnMap map[u.Key]*Conn

// Dial connects to a particular peer, over a given network
// Example: Dial("udp", peer)
func Dial(network string, peer *peer.Peer) (*Conn, error) {
	addr := peer.NetAddress(network)
	if addr == nil {
		return nil, fmt.Errorf("No address for network %s", network)
	}

	network, host, err := addr.DialArgs()
	if err != nil {
		return nil, err
	}

	nconn, err := net.Dial(network, host)
	if err != nil {
		return nil, err
	}

	out := msgio.NewChan(10)
	inc := msgio.NewChan(10)

	conn := &Conn{
		Peer: peer,
		Addr: addr,
		Conn: nconn,

		Outgoing: out,
		Incoming: inc,
		Closed:   make(chan bool, 1),
	}

	go out.WriteTo(nconn)
	go inc.ReadFrom(nconn, 1<<12)

	return conn, nil
}

// Close closes the connection, and associated channels.
func (s *Conn) Close() error {
	if s.Conn == nil {
		return fmt.Errorf("Already closed.") // already closed
	}

	// closing net connection
	err := s.Conn.Close()
	s.Conn = nil
	// closing channels
	s.Incoming.Close()
	s.Outgoing.Close()
	s.Closed <- true
	return err
}
