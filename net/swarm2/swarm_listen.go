package swarm

import (
	"fmt"

	conn "github.com/jbenet/go-ipfs/net/conn"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	multierr "github.com/jbenet/go-ipfs/util/multierr"
	ps "github.com/jbenet/go-peerstream"
)

// Open listeners for each network the swarm should listen on
func (s *Swarm) listen(addrs []ma.Multiaddr) error {
	retErr := multierr.New()

	// listen on every address
	for i, addr := range addrs {
		err := s.setupListener(addr)
		if err != nil {
			retErr.Errors[i] = err
			log.Errorf("Failed to listen on: %s - %s", addr, err)
		}
	}

	if len(retErr.Errors) > 0 {
		return retErr
	}
	return nil
}

// Listen for new connections on the given multiaddr
func (s *Swarm) setupListener(maddr ma.Multiaddr) error {

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

	// AddListener to the peerstream Listener. this will begin accepting connections
	// and streams!
	_, err = s.swarm.AddListener(list)
	return err
}

// connHandler is called by the StreamSwarm whenever a new connection is added
// here we configure it slightly. Note that this is sequential, so if anything
// will take a while do it in a goroutine.
// See https://godoc.org/github.com/jbenet/go-peerstream for more information
func (s *Swarm) connHandler(c1 *ps.Conn) {

	// grab the underlying connection.
	if c2, ok := c1.NetConn().(conn.Conn); ok {

		// set the RemotePeer as a group on the conn. this lets us group
		// connections in the StreamSwarm by peer, and get a streams from
		// any available connection in the group (better multiconn):
		//   swarm.StreamSwarm().NewStreamWithGroup(remotePeer)
		c1.AddGroup(c2.RemotePeer())

		go func() {
			ctx := context.Background()
			err := runHandshake3(ctx, s, c1, c2)
			if err != nil {
				log.Error("Handshake3 failed. disconnecting", err)
				log.Event(ctx, "Handshake3FailureDisconnect", c2.LocalPeer(), c2.RemotePeer())
				c1.Close() // boom.
			}
		}()
	}
}

func runHandshake3(ctx context.Context, s *Swarm, sc *ps.Conn, c conn.Conn) error {
	log.Event(ctx, "newConnection", c.LocalPeer(), c.RemotePeer())

	stream, err := sc.NewStream()
	if err != nil {
		return err
	}

	// handshake3
	h3result, err := conn.Handshake3(ctx, stream, c)
	if err != nil {
		return fmt.Errorf("Handshake3 failed: %s", err)
	}

	// check for nats. you know, just in case.
	if h3result.LocalObservedAddress != nil {
		checkNATWarning(s, h3result.LocalObservedAddress, c.LocalMultiaddr())
	} else {
		log.Warningf("Received nil observed address from %s", c.RemotePeer())
	}

	stream.Close()
	log.Event(ctx, "handshake3Succeeded", c.LocalPeer(), c.RemotePeer())
	return nil
}
