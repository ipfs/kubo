package swarm

import (
	conn "github.com/jbenet/go-ipfs/net/conn"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"

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
			if retErr.Errors == nil {
				retErr.Errors = make([]error, len(addrs))
			}
			retErr.Errors[i] = err
			log.Errorf("Failed to listen on: %s - %s", addr, err)
		}
	}

	if retErr.Errors != nil {
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
func (s *Swarm) connHandler(c *ps.Conn) {
	go func() {
		ctx := context.Background()
		// this context is for running the handshake, which -- when receiveing connections
		// -- we have no bound on beyond what the transport protocol bounds it at.
		// note that setup + the handshake are bounded by underlying io.
		// (i.e. if TCP or UDP disconnects (or the swarm closes), we're done.
		// Q: why not have a shorter handshake? think about an HTTP server on really slow conns.
		// as long as the conn is live (TCP says its online), it tries its best. we follow suit.)

		if _, err := s.newConnSetup(ctx, c); err != nil {
			log.Error(err)
			log.Event(ctx, "newConnHandlerDisconnect", lgbl.NetConn(c.NetConn()), lgbl.Error(err))
			c.Close() // boom. close it.
			return
		}
	}()
}
