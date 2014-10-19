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

	list, err := conn.Listen(s.Context(), maddr, s.local, s.peers)
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
	// this listener is a child.
	s.Children().Add(1)
	go func() {
		defer s.Children().Done()

		for {
			select {
			case <-s.Closing():
				return

			case conn := <-list.Accept():
				// handler also a child.
				s.Children().Add(1)
				go s.handleIncomingConn(conn)
			}
		}
	}()

	return nil
}

// Handle getting ID from this peer, handshake, and adding it into the map
func (s *Swarm) handleIncomingConn(nconn conn.Conn) {
	// this handler is a child. added by caller.
	defer s.Children().Done()

	// Setup the new connection
	_, err := s.connSetup(nconn)
	if err != nil && err != ErrAlreadyOpen {
		s.errChan <- err
		nconn.Close()
	}
}

// connSetup adds the passed in connection to its peerMap and starts
// the fanInSingle routine for that connection
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

	mc, ok := s.conns[c.RemotePeer().Key()]
	if !ok {
		// multiconn doesn't exist, make a new one.
		conns := []conn.Conn{c}
		mc, err := conn.NewMultiConn(s.Context(), s.local, c.RemotePeer(), conns)
		if err != nil {
			log.Error("error creating multiconn: %s", err)
			c.Close()
			return nil, err
		}

		s.conns[c.RemotePeer().Key()] = mc
		s.connsLock.Unlock()

		log.Debug("added new multiconn: %s", mc)
	} else {
		s.connsLock.Unlock() // unlock before adding new conn

		mc.Add(c)
		log.Debug("multiconn found: %s", mc)
	}

	log.Debug("multiconn added new conn %s", c)

	// kick off reader goroutine
	go s.fanInSingle(c)
	return c, nil
}

// Handles the unwrapping + sending of messages to the right connection.
func (s *Swarm) fanOut() {
	s.Children().Add(1)
	defer s.Children().Done()

	for {
		select {
		case <-s.Closing():
			return // told to close.

		case msg, ok := <-s.Outgoing:
			if !ok {
				return
			}

			s.connsLock.RLock()
			c, found := s.conns[msg.Peer().Key()]
			s.connsLock.RUnlock()

			if !found {
				e := fmt.Errorf("Sent msg to peer without open conn: %v", msg.Peer())
				s.errChan <- e
				log.Error("%s", e)
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
func (s *Swarm) fanInSingle(c conn.Conn) {
	s.Children().Add(1)
	c.Children().Add(1) // child of Conn as well.

	// cleanup all data associated with this child Connection.
	defer func() {
		// remove it from the map.
		s.connsLock.Lock()
		delete(s.conns, c.RemotePeer().Key())
		s.connsLock.Unlock()

		s.Children().Done()
		c.Children().Done() // child of Conn as well.
	}()

	for {
		select {
		case <-s.Closing(): // Swarm closing
			return

		case <-c.Closing(): // Conn closing
			return

		case data, ok := <-c.In():
			if !ok {
				return // channel closed.
			}
			// log.Debug("[peer: %s] Received message [from = %s]", s.local, c.Peer)
			s.Incoming <- msg.New(c.RemotePeer(), data)
		}
	}
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
