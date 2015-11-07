package swarm

import (
	mconn "github.com/ipfs/go-ipfs/metrics/conn"
	inet "github.com/ipfs/go-ipfs/p2p/net"
	conn "github.com/ipfs/go-ipfs/p2p/net/conn"
	lgbl "github.com/ipfs/go-ipfs/util/eventlog/loggables"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	ps "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	multierr "github.com/ipfs/go-ipfs/thirdparty/multierr"
)

// Open listeners and reuse-dialers for the given addresses
func (s *Swarm) setupAddresses(addrs []ma.Multiaddr) error {
	merr := multierr.New()
	for i, a := range addrs {
		t, err := conn.MakeTransport(a, DialTimeout)
		if err != nil {
			if err == conn.ErrNoSpecialTransport {
				log.Error("no special transport for ", a)
				continue
			}
			if merr.Errors == nil {
				merr.Errors = make([]error, len(addrs))
			}
			merr.Errors[i] = err
			continue
		}
		s.dialer.AddDialer(t)

		err = s.addListener(t)
		if err != nil {
			merr.Errors[i] = err
		}
	}

	if merr.Errors != nil {
		return merr
	}

	return nil
}

func (s *Swarm) addListener(malist manet.Listener) error {

	sk := s.peers.PrivKey(s.local)
	if sk == nil {
		// may be fine for sk to be nil, just log a warning.
		log.Warning("Listener not given PrivateKey, so WILL NOT SECURE conns.")
	}

	list, err := conn.WrapManetListener(s.Context(), malist, s.local, sk)
	if err != nil {
		return err
	}

	list.SetAddrFilters(s.Filters)

	if cw, ok := list.(conn.ListenerConnWrapper); ok {
		cw.SetConnWrapper(func(c manet.Conn) manet.Conn {
			return mconn.WrapConn(s.bwc, c)
		})
	}

	return s.addConnListener(list)
}

func (s *Swarm) addConnListener(list conn.Listener) error {
	// AddListener to the peerstream Listener. this will begin accepting connections
	// and streams!
	sl, err := s.swarm.AddListener(list)
	if err != nil {
		return err
	}
	log.Debugf("Swarm Listeners at %s", s.ListenAddresses())

	maddr := list.Multiaddr()

	// signal to our notifiees on successful conn.
	s.notifyAll(func(n inet.Notifiee) {
		n.Listen((*Network)(s), maddr)
	})

	// go consume peerstream's listen accept errors. note, these ARE errors.
	// they may be killing the listener, and if we get _any_ we should be
	// fixing this in our conn.Listener (to ignore them or handle them
	// differently.)
	go func(ctx context.Context, sl *ps.Listener) {

		// signal to our notifiees closing
		defer s.notifyAll(func(n inet.Notifiee) {
			n.ListenClose((*Network)(s), maddr)
		})

		for {
			select {
			case err, more := <-sl.AcceptErrors():
				if !more {
					return
				}
				log.Errorf("swarm listener accept error: %s", err)
			case <-ctx.Done():
				return
			}
		}
	}(s.Context(), sl)

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
		log.Debug(err)
		log.Event(ctx, "newConnHandlerDisconnect", lgbl.NetConn(c.NetConn()), lgbl.Error(err))
		c.Close() // boom. close it.
		return nil
	}

	return sc
}
