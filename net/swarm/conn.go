package swarm

import (
	"errors"
	"fmt"
	"net"

	ident "github.com/jbenet/go-ipfs/identify"
	conn "github.com/jbenet/go-ipfs/net/conn"

	u "github.com/jbenet/go-ipfs/util"
)

// Handle getting ID from this peer, handshake, and adding it into the map
func (s *Swarm) handleIncomingConn(nconn net.Conn) {

	c, err := conn.NewNetConn(nconn)
	if err != nil {
		s.errChan <- err
		return
	}

	//TODO(jbenet) the peer might potentially already be in the global PeerBook.
	// maybe use the handshake to populate peer.
	c.Peer.AddAddress(c.Addr)

	// Setup the new connection
	err = s.connSetup(c)
	if err != nil && err != ErrAlreadyOpen {
		s.errChan <- err
		c.Close()
	}
}

// connSetup adds the passed in connection to its peerMap and starts
// the fanIn routine for that connection
func (s *Swarm) connSetup(c *conn.Conn) error {
	if c == nil {
		return errors.New("Tried to start nil connection.")
	}

	u.DOut("Starting connection: %s\n", c.Peer.Key().Pretty())

	// handshake
	if err := s.connHandshake(c); err != nil {
		return fmt.Errorf("Conn handshake error: %v", err)
	}

	// add to conns
	s.connsLock.Lock()
	if _, ok := s.conns[c.Peer.Key()]; ok {
		s.connsLock.Unlock()
		return ErrAlreadyOpen
	}
	s.conns[c.Peer.Key()] = c
	s.connsLock.Unlock()

	// kick off reader goroutine
	go s.fanIn(c)
	return nil
}

// connHandshake runs the handshake with the remote connection.
func (s *Swarm) connHandshake(c *conn.Conn) error {

	//TODO(jbenet) this Handshake stuff should be moved elsewhere.
	// needs cleanup. needs context. use msg.Pipe.
	return ident.Handshake(s.local, c.Peer, c.Incoming.MsgChan, c.Outgoing.MsgChan)
}
