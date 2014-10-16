package swarm

import (
	"errors"
	"fmt"

	conn "github.com/jbenet/go-ipfs/net/conn"
	handshake "github.com/jbenet/go-ipfs/net/handshake"
	msg "github.com/jbenet/go-ipfs/net/message"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
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
	err := s.connSetup(nconn)
	if err != nil && err != ErrAlreadyOpen {
		s.errChan <- err
		nconn.Close()
	}
}

// connSetup adds the passed in connection to its peerMap and starts
// the fanIn routine for that connection
func (s *Swarm) connSetup(c conn.Conn) error {
	if c == nil {
		return errors.New("Tried to start nil connection.")
	}

	log.Debug("%s Started connection: %s", c.LocalPeer(), c.RemotePeer())

	// add address of connection to Peer. Maybe it should happen in connSecure.
	// NOT adding this address here, because the incoming address in TCP
	// is an EPHEMERAL address, and not the address we want to keep around.
	// addresses should be figured out through the DHT.
	// c.Remote.AddAddress(c.Conn.RemoteMultiaddr())

	if err := s.connVersionExchange(c); err != nil {
		return fmt.Errorf("Conn version exchange error: %v", err)
	}

	// add to conns
	s.connsLock.Lock()
	if _, ok := s.conns[c.RemotePeer().Key()]; ok {
		log.Debug("Conn already open!")
		s.connsLock.Unlock()
		return ErrAlreadyOpen
	}
	s.conns[c.RemotePeer().Key()] = c
	log.Debug("Added conn to map!")
	s.connsLock.Unlock()

	// kick off reader goroutine
	go s.fanIn(c)
	return nil
}

// connVersionExchange exchanges local and remote versions and compares them
// closes remote and returns an error in case of major difference
func (s *Swarm) connVersionExchange(r conn.Conn) error {
	rpeer := r.RemotePeer()

	var remoteH, localH *handshake.Handshake1
	localH = handshake.CurrentHandshake()

	myVerBytes, err := proto.Marshal(localH)
	if err != nil {
		return err
	}

	r.MsgOut() <- msg.New(rpeer, myVerBytes)
	log.Debug("Sent my version(%s) [to = %s]", localH, rpeer)

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()

	// case <-remote.Done():
	// 	return errors.New("remote closed connection during version exchange")

	case data, ok := <-r.MsgIn():
		if !ok {
			return fmt.Errorf("Error retrieving from conn: %v", rpeer)
		}

		remoteH = new(handshake.Handshake1)
		err = proto.Unmarshal(data.Data(), remoteH)
		if err != nil {
			s.Close()
			return fmt.Errorf("connSetup: could not decode remote version: %q", err)
		}

		log.Debug("Received remote version(%s) [from = %s]", remoteH, rpeer)
	}

	if err := handshake.Compatible(localH, remoteH); err != nil {
		log.Info("%s (%s) incompatible version with %s (%s)", s.local, localH, rpeer, remoteH)
		r.Close()
		return err
	}

	log.Debug("[peer: %s] Version compatible", rpeer)
	return nil
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
			conn, found := s.conns[msg.Peer().Key()]
			s.connsLock.RUnlock()

			if !found {
				e := fmt.Errorf("Sent msg to peer without open conn: %v",
					msg.Peer)
				s.errChan <- e
				continue
			}

			// log.Debug("[peer: %s] Sent message [to = %s]", s.local, msg.Peer())

			// queue it in the connection's buffer
			conn.MsgOut() <- msg
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

		case data, ok := <-c.MsgIn():
			if !ok {
				e := fmt.Errorf("Error retrieving from conn: %v", c.RemotePeer())
				s.errChan <- e
				goto out
			}

			// log.Debug("[peer: %s] Received message [from = %s]", s.local, c.Peer)
			s.Incoming <- data
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
