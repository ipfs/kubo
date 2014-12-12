package swarm

import (
	"errors"
	"fmt"

	conn "github.com/jbenet/go-ipfs/net/conn"
	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// Open listeners for each network the swarm should listen on
func (s *Swarm) listen(addrs []ma.Multiaddr) error {
	hasErr := false
	retErr := &ListenErr{
		Errors: make([]error, len(addrs)),
	}

	// listen on every address
	for i, addr := range addrs {
		err := s.connListen(addr)
		if err != nil {
			hasErr = true
			retErr.Errors[i] = err
			log.Errorf("Failed to listen on: %s - %s", addr, err)
		}
	}

	if hasErr {
		return retErr
	}
	return nil
}

// Listen for new connections on the given multiaddr
func (s *Swarm) connListen(maddr ma.Multiaddr) error {

	resolved, err := resolveUnspecifiedAddresses([]ma.Multiaddr{maddr})
	if err != nil {
		return err
	}

	list, err := conn.Listen(s.Context(), maddr, s.local, s.peers)
	if err != nil {
		return err
	}

	// add resolved local addresses to peer
	for _, addr := range resolved {
		s.local.AddAddress(addr)
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

// peerMultiConn returns the MultiConn responsible for handling this peer.
// if there is none, it creates one and returns it. Note that timeouts
// and connection teardowns will remove it.
func (s *Swarm) peerMultiConn(p peer.Peer) (*conn.MultiConn, error) {

	s.connsLock.Lock()
	mc, found := s.conns[p.Key()]
	if found {
		s.connsLock.Unlock()
		return mc, nil
	}

	// multiconn doesn't exist, make a new one.
	mc, err := conn.NewMultiConn(s.Context(), s.local, p, nil)
	if err != nil {
		s.connsLock.Unlock()
		log.Errorf("error creating multiconn: %s", err)
		return nil, err
	}
	s.conns[p.Key()] = mc
	s.connsLock.Unlock()

	// kick off reader goroutine
	s.Children().Add(1)
	mc.Children().Add(1) // child of Conn as well.
	go s.fanInSingle(mc)
	return mc, nil
}

// connSetup takes a new connection, performs the IPFS handshake (handshake3)
// and then adds it to the appropriate MultiConn.
func (s *Swarm) connSetup(c conn.Conn) (conn.Conn, error) {
	if c == nil {
		return nil, errors.New("Tried to start nil connection.")
	}

	log.Event(context.TODO(), "connSetupBegin", c.LocalPeer(), c.RemotePeer())

	// add address of connection to Peer. Maybe it should happen in connSecure.
	// NOT adding this address here, because the incoming address in TCP
	// is an EPHEMERAL address, and not the address we want to keep around.
	// addresses should be figured out through the DHT.
	// c.Remote.AddAddress(c.Conn.RemoteMultiaddr())

	// handshake3
	ctxT, _ := context.WithTimeout(c.Context(), conn.HandshakeTimeout)
	h3result, err := conn.Handshake3(ctxT, c)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("Handshake3 failed: %s", err)
	}

	// check for nats. you know, just in case.
	if h3result.LocalObservedAddress != nil {
		s.checkNATWarning(h3result.LocalObservedAddress)
	} else {
		log.Warningf("Received nil observed address from %s", c.RemotePeer())
	}

	// add to conns
	mc, err := s.peerMultiConn(c.RemotePeer())
	if err != nil {
		c.Close()
		return nil, err
	}
	mc.Add(c)
	log.Event(context.TODO(), "connSetupSuccess", c.LocalPeer(), c.RemotePeer())
	return c, nil
}

// Handles the unwrapping + sending of messages to the right connection.
func (s *Swarm) fanOut() {
	defer s.Children().Done()

	i := 0
	for {
		select {
		case <-s.Closing():
			return // told to close.

		case msg, ok := <-s.Outgoing:
			if !ok {
				log.Infof("%s outgoing channel closed", s)
				return
			}
			if len(msg.Data()) >= conn.MaxMessageSize {
				log.Criticalf("Attempted to send message bigger than max size. (%d)", len(msg.Data()))
			}

			s.connsLock.RLock()
			c, found := s.conns[msg.Peer().Key()]
			s.connsLock.RUnlock()

			if !found {
				e := fmt.Errorf("Sent msg to peer without open conn: %v", msg.Peer())
				s.errChan <- e
				log.Error(e)
				continue
			}

			i++
			log.Debugf("%s sent message to %s (%d)", s.local, msg.Peer(), i)
			log.Event(context.TODO(), "sendMessage", s.local, msg)
			// queue it in the connection's buffer
			if err := c.WriteMsg(msg.Data()); err != nil {
				log.Infof("%s connection failed to write: %s", c, err)
				continue
			}
		}
	}
}

// Handles the receiving + wrapping of messages, per conn.
// Consider using reflect.Select with one goroutine instead of n.
func (s *Swarm) fanInSingle(c conn.Conn) {
	// cleanup all data associated with this child Connection.
	defer func() {
		// remove it from the map.
		s.connsLock.Lock()
		delete(s.conns, c.RemotePeer().Key())
		s.connsLock.Unlock()

		s.Children().Done()
		c.Children().Done() // child of Conn as well.
	}()

	// use readChan to be able to listen to Closing events
	rchan := readChan(s.Context(), c)

	i := 0
	for {
		select {
		case <-s.Closing(): // Swarm closing
			return

		case <-c.Closing(): // Conn closing
			return

		case data, ok := <-rchan:
			if !ok {
				log.Infof("%s in channel closed", c)
				return // channel closed.
			}
			i++
			log.Debugf("%s received message from %s (%d)", s.local, c.RemotePeer(), i)
			s.Incoming <- msg.New(c.RemotePeer(), data)
		}
	}
}

// readChan is a temporary fixture to match the old interface. will be removed soon.
func readChan(ctx context.Context, c conn.Conn) <-chan []byte {

	ch := make(chan []byte) // no buffer. sync.

	go func() {
		defer close(ch)

		for {
			msg, err := c.ReadMsg()
			if err != nil {
				log.Infof("%s connection failed: %s", c, err)
				return
			}

			select {
			case <-c.Closing():
				return
			case <-ctx.Done():
				return
			case ch <- msg:
			}
		}
	}()

	return ch
}
