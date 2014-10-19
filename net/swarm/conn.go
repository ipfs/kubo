package swarm

import (
	"errors"
	"fmt"

	conn "github.com/jbenet/go-ipfs/net/conn"
	msg "github.com/jbenet/go-ipfs/net/message"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
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
			log.Error("Failed to listen on: %s - %s", addr, err)
		}
	}

	if hasErr {
		return retErr
	}
	return nil
}

// Listen for new connections on the given multiaddr
func (s *Swarm) connListen(maddr ma.Multiaddr) error {

	list, err := conn.Listen(s.ctx, maddr, s.local, s.peers)
	if err != nil {
		return err
	}

	// make sure port can be reused. TOOD this doesn't work...
	// if err := setSocketReuse(list); err != nil {
	// 	return err
	// }

	// NOTE: this may require a lock around it later. currently, only run on setup
	s.listeners = append(s.listeners, list)

	// Accept and handle new connections on this listener until it errors
	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return

			case conn := <-list.Accept():
				go s.handleIncomingConn(conn)
			}
		}
	}()

	return nil
}

// Handle getting ID from this peer, handshake, and adding it into the map
func (s *Swarm) handleIncomingConn(nconn conn.Conn) {

	// Setup the new connection
	_, err := s.connSetup(nconn)
	if err != nil && err != ErrAlreadyOpen {
		s.errChan <- err
		nconn.Close()
	}
}

// connSetup adds the passed in connection to its peerMap and starts
// the fanIn routine for that connection
func (s *Swarm) connSetup(c conn.Conn) (conn.Conn, error) {
	if c == nil {
		return nil, errors.New("Tried to start nil connection.")
	}

	log.Debug("%s Started connection: %s", c.LocalPeer(), c.RemotePeer())

	// add address of connection to Peer. Maybe it should happen in connSecure.
	// NOT adding this address here, because the incoming address in TCP
	// is an EPHEMERAL address, and not the address we want to keep around.
	// addresses should be figured out through the DHT.
	// c.Remote.AddAddress(c.Conn.RemoteMultiaddr())

	// add to conns
	s.connsLock.Lock()
	if c2, ok := s.conns[c.RemotePeer().Key()]; ok {
		log.Debug("Conn already open!")
		s.connsLock.Unlock()

		c.Close()
		return c2, nil // not error anymore, use existing conn.
		// return ErrAlreadyOpen
	}
	s.conns[c.RemotePeer().Key()] = c
	log.Debug("Added conn to map!")
	s.connsLock.Unlock()

	// kick off reader goroutine
	go s.fanIn(c)
	return c, nil
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
			c, found := s.conns[msg.Peer().Key()]
			s.connsLock.RUnlock()

			if !found {
				e := fmt.Errorf("Sent msg to peer without open conn: %v",
					msg.Peer)
				s.errChan <- e
				continue
			}

			// log.Debug("[peer: %s] Sent message [to = %s]", s.local, msg.Peer())

			// queue it in the connection's buffer
			c.Out() <- msg.Data()
		}
	}
}

// Handles the receiving + wrapping of messages, per conn.
// Consider using reflect.Select with one goroutine instead of n.
func (s *Swarm) fanIn(c conn.Conn) {
	for {
		select {
		case <-s.ctx.Done():
			// close Conn.
			c.Close()
			goto out

		case data, ok := <-c.In():
			if !ok {
				e := fmt.Errorf("Error retrieving from conn: %v", c.RemotePeer())
				s.errChan <- e
				goto out
			}

			// log.Debug("[peer: %s] Received message [from = %s]", s.local, c.Peer)
			s.Incoming <- msg.New(c.RemotePeer(), data)
		}
	}

out:
	s.connsLock.Lock()
	delete(s.conns, c.RemotePeer().Key())
	s.connsLock.Unlock()
}

// Commenting out because it's platform specific
// func setSocketReuse(l manet.Listener) error {
// 	nl := l.NetListener()
//
// 	// for now only TCP. TODO change this when more networks.
// 	file, err := nl.(*net.TCPListener).File()
// 	if err != nil {
// 		return err
// 	}
//
// 	fd := file.Fd()
// 	err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
// 	if err != nil {
// 		return err
// 	}
//
// 	err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
// 	if err != nil {
// 		return err
// 	}
//
// 	return nil
// }
