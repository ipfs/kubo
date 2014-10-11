package swarm

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
	conn "github.com/jbenet/go-ipfs/net/conn"
	handshake "github.com/jbenet/go-ipfs/net/handshake"
	msg "github.com/jbenet/go-ipfs/net/message"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"
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
	list, err := manet.Listen(maddr)
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
			nconn, err := list.Accept()
			if err != nil {
				e := fmt.Errorf("Failed to accept connection: %s - %s", maddr, err)
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
func (s *Swarm) handleIncomingConn(nconn manet.Conn) {

	// Construct conn with nil peer for now, because we don't know its ID yet.
	// connSetup will figure this out, and pull out / construct the peer.
	c, err := conn.NewConn(s.local, nil, nconn)
	if err != nil {
		s.errChan <- err
		return
	}

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

	if c.Remote != nil {
		log.Debug("%s Starting connection: %s", c.Local, c.Remote)
	} else {
		log.Debug("%s Starting connection: [unknown peer]", c.Local)
	}

	if err := s.connSecure(c); err != nil {
		return fmt.Errorf("Conn securing error: %v", err)
	}

	log.Debug("%s secured connection: %s", c.Local, c.Remote)

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
	if _, ok := s.conns[c.Remote.Key()]; ok {
		log.Debug("Conn already open!")
		s.connsLock.Unlock()
		return ErrAlreadyOpen
	}
	s.conns[c.Remote.Key()] = c
	log.Debug("Added conn to map!")
	s.connsLock.Unlock()

	// kick off reader goroutine
	go s.fanIn(c)
	return nil
}

// connSecure setups a secure remote connection.
func (s *Swarm) connSecure(c *conn.Conn) error {

	sp, err := spipe.NewSecurePipe(s.ctx, 10, s.local, s.peers)
	if err != nil {
		return err
	}

	err = sp.Wrap(s.ctx, spipe.Duplex{
		In:  c.Incoming.MsgChan,
		Out: c.Outgoing.MsgChan,
	})
	if err != nil {
		return err
	}

	if c.Remote == nil {
		c.Remote = sp.RemotePeer()

	} else if c.Remote != sp.RemotePeer() {
		panic("peers not being constructed correctly.")
	}

	c.Secure = sp
	return nil
}

// connVersionExchange exchanges local and remote versions and compares them
// closes remote and returns an error in case of major difference
func (s *Swarm) connVersionExchange(remote *conn.Conn) error {
	var remoteHandshake, localHandshake *handshake.Handshake1
	localHandshake = handshake.CurrentHandshake()

	myVerBytes, err := proto.Marshal(localHandshake)
	if err != nil {
		return err
	}

	remote.Secure.Out <- myVerBytes

	log.Debug("Send my version(%s) [to = %s]", localHandshake, remote.Peer)

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()

	case <-remote.Closed:
		return errors.New("remote closed connection during version exchange")

	case data, ok := <-remote.Secure.In:
		if !ok {
			return fmt.Errorf("Error retrieving from conn: %v", remote.Peer)
		}

		remoteHandshake = new(handshake.Handshake1)
		err = proto.Unmarshal(data, remoteHandshake)
		if err != nil {
			s.Close()
			return fmt.Errorf("connSetup: could not decode remote version: %q", err)
		}

		log.Debug("Received remote version(%s) [from = %s]", remoteHandshake, remote.Peer)
	}

	if err := handshake.Compatible(localHandshake, remoteHandshake); err != nil {
		log.Info("%s (%s) incompatible version with %s (%s)", s.local, localHandshake, remote.Peer, remoteHandshake)
		remote.Close()
		return err
	}

	log.Debug("[peer: %s] Version compatible", remote.Peer)
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
			conn.Secure.Out <- msg.Data()
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

		case data, ok := <-c.Secure.In:
			if !ok {
				e := fmt.Errorf("Error retrieving from conn: %v", c.Remote)
				s.errChan <- e
				goto out
			}

			// log.Debug("[peer: %s] Received message [from = %s]", s.local, c.Peer)

			msg := msg.New(c.Remote, data)
			s.Incoming <- msg
		}
	}

out:
	s.connsLock.Lock()
	delete(s.conns, c.Remote.Key())
	s.connsLock.Unlock()
}

func setSocketReuse(l manet.Listener) error {
	nl := l.NetListener()

	// for now only TCP. TODO change this when more networks.
	file, err := nl.(*net.TCPListener).File()
	if err != nil {
		return err
	}

	fd := file.Fd()
	err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		return err
	}

	err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
	if err != nil {
		return err
	}

	return nil
}
