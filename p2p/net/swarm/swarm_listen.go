package swarm

import (
	"fmt"

	conn "github.com/jbenet/go-ipfs/p2p/net/conn"
	addrutil "github.com/jbenet/go-ipfs/p2p/net/swarm/addr"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	ps "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream"
	multierr "github.com/jbenet/go-ipfs/util/multierr"
)

// Open listeners for each network the swarm should listen on
func (s *Swarm) listen(addrs []ma.Multiaddr) error {

	for _, addr := range addrs {
		if !addrutil.AddrUsable(addr, true) {
			return fmt.Errorf("cannot use addr: %s", addr)
		}
	}

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

	// TODO rethink how this has to work. (jbenet)
	//
	// resolved, err := resolveUnspecifiedAddresses([]ma.Multiaddr{maddr})
	// if err != nil {
	// 	return err
	// }
	// for _, a := range resolved {
	// 	s.peers.AddAddress(s.local, a)
	// }

	sk := s.peers.PrivKey(s.local)
	if sk == nil {
		// may be fine for sk to be nil, just log a warning.
		log.Warning("Listener not given PrivateKey, so WILL NOT SECURE conns.")
	}
	log.Infof("Swarm Listening at %s", maddr)
	list, err := conn.Listen(s.cg.Context(), maddr, s.local, sk)
	if err != nil {
		return err
	}
	log.Infof("Swarm Listening at %s", s.ListenAddresses())

	// AddListener to the peerstream Listener. this will begin accepting connections
	// and streams!
	sl, err := s.swarm.AddListener(list)
	if err != nil {
		return err
	}

	// go consume peerstream's listen accept errors. note, these ARE errors.
	// they may be killing the listener, and if we get _any_ we should be
	// fixing this in our conn.Listener (to ignore them or handle them
	// differently.)
	go func(ctx context.Context, sl *ps.Listener) {
		for {
			select {
			case err, more := <-sl.AcceptErrors():
				if !more {
					return
				}
				log.Info(err)
			case <-ctx.Done():
				return
			}
		}
	}(s.cg.Context(), sl)

	return nil
}

// connHandler is called by the StreamSwarm whenever a new connection is added
// here we configure it slightly. Note that this is sequential, so if anything
// will take a while do it in a goroutine.
// See https://godoc.org/github.com/jbenet/go-peerstream for more information
func (s *Swarm) connHandler(c *ps.Conn) *Conn {
	ctx := context.Background()
	// this context is for running the handshake, which -- when receiveing connections
	// -- we have no bound on beyond what the transport protocol bounds it at.
	// note that setup + the handshake are bounded by underlying io.
	// (i.e. if TCP or UDP disconnects (or the swarm closes), we're done.
	// Q: why not have a shorter handshake? think about an HTTP server on really slow conns.
	// as long as the conn is live (TCP says its online), it tries its best. we follow suit.)

	sc, err := s.newConnSetup(ctx, c)
	if err != nil {
		log.Error(err)
		log.Event(ctx, "newConnHandlerDisconnect", lgbl.NetConn(c.NetConn()), lgbl.Error(err))
		c.Close() // boom. close it.
		return nil
	}

	return sc
}
