package swarm

import (
	"errors"
	"fmt"

	conn "github.com/jbenet/go-ipfs/net/conn"

	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
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

	list, err := conn.Listen(s.cg.Context(), maddr, s.local, s.peers)
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
	s.cg.AddChildFunc(func(parent ctxgroup.ContextGroup) {
		for {
			select {
			case <-parent.Closing():
				return

			case conn := <-list.Accept():
				s.handleIncomingConn(parent.Context(), conn)
			}
		}
	})

	return nil
}

// Handle getting ID from this peer, handshake, and adding it into the map
func (s *Swarm) handleIncomingConn(ctx context.Context, nconn conn.Conn) {
	// Setup the new connection
	_, err := s.connSetup(ctx, nconn)
	if err != nil {
		log.Errorf("swarm: failed to add incoming connection: %s", err)
		log.Event(ctx, "handleIncomingConn failed", s.LocalPeer(), nconn.RemotePeer())
		nconn.Close()
	}
}

// connSetup takes a new connection, performs the IPFS handshake (handshake3)
// and then adds it to the appropriate MultiConn.
func (s *Swarm) connSetup(ctx context.Context, c conn.Conn) (conn.Conn, error) {
	if c == nil {
		return nil, errors.New("Tried to start nil connection.")
	}

	log.Event(ctx, "connSetupBegin", c.LocalPeer(), c.RemotePeer())

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
		s.checkNATWarning(h3result.LocalObservedAddress, c.LocalMultiaddr())
	} else {
		log.Warningf("Received nil observed address from %s", c.RemotePeer())
	}

	// add to conns
	if err := s.addConn(c); err != nil {
		c.Close()
		return nil, err
	}
	log.Event(ctx, "connSetupSuccess", c.LocalPeer(), c.RemotePeer())
	return c, nil
}
