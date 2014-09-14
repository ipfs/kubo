package swarm

import (
	"errors"
	"fmt"
	"net"

	ident "github.com/jbenet/go-ipfs/identify"
	conn "github.com/jbenet/go-ipfs/net/conn"
	msg "github.com/jbenet/go-ipfs/net/message"
	u "github.com/jbenet/go-ipfs/util"

	ma "github.com/jbenet/go-multiaddr"
)

// Open listeners for each network the swarm should listen on
func (s *Swarm) listen() error {
	hasErr := false
	retErr := &ListenErr{
		Errors: make([]error, len(s.local.Addresses)),
	}

	// listen on every address
	for i, addr := range s.local.Addresses {
		err := s.connListen(addr)
		if err != nil {
			hasErr = true
			retErr.Errors[i] = err
			u.PErr("Failed to listen on: %s [%s]", addr, err)
		}
	}

	if hasErr {
		return retErr
	}
	return nil
}

// Listen for new connections on the given multiaddr
func (s *Swarm) connListen(maddr *ma.Multiaddr) error {
	netstr, addr, err := maddr.DialArgs()
	if err != nil {
		return err
	}

	list, err := net.Listen(netstr, addr)
	if err != nil {
		return err
	}

	// NOTE: this may require a lock around it later. currently, only run on setup
	s.listeners = append(s.listeners, list)

	// Accept and handle new connections on this listener until it errors
	go func() {
		for {
			nconn, err := list.Accept()
			if err != nil {
				e := fmt.Errorf("Failed to accept connection: %s - %s [%s]",
					netstr, addr, err)
				s.errChan <- e

				// if cancel is nil, we're closed.
				if s.cancel == nil {
					return
				}
			} else {
				go s.handleIncomingConn(nconn)
			}
		}
	}()

	return nil
}

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

// Handles the unwrapping + sending of messages to the right connection.
func (s *Swarm) fanOut() {
	for {
		select {
		case <-s.ctx.Done():
			return // told to close.

		case msg, ok := <-s.Outgoing:
			if !ok {
				return
			}

			s.connsLock.RLock()
			conn, found := s.conns[msg.Peer.Key()]
			s.connsLock.RUnlock()

			if !found {
				e := fmt.Errorf("Sent msg to peer without open conn: %v",
					msg.Peer)
				s.errChan <- e
				continue
			}

			// queue it in the connection's buffer
			conn.Outgoing.MsgChan <- msg.Data
		}
	}
}

// Handles the receiving + wrapping of messages, per conn.
// Consider using reflect.Select with one goroutine instead of n.
func (s *Swarm) fanIn(c *conn.Conn) {
	for {
		select {
		case <-s.ctx.Done():
			// close Conn.
			c.Close()
			goto out

		case <-c.Closed:
			goto out

		case data, ok := <-c.Incoming.MsgChan:
			if !ok {
				e := fmt.Errorf("Error retrieving from conn: %v", c.Peer.Key().Pretty())
				s.errChan <- e
				goto out
			}

			msg := &msg.Message{Peer: c.Peer, Data: data}
			s.Incoming <- msg
		}
	}

out:
	s.connsLock.Lock()
	delete(s.conns, c.Peer.Key())
	s.connsLock.Unlock()
}
