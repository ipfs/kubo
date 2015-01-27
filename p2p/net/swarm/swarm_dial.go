package swarm

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	conn "github.com/jbenet/go-ipfs/p2p/net/conn"
	addrutil "github.com/jbenet/go-ipfs/p2p/net/swarm/addr"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
)

// Diagram of dial sync:
//
//   many callers of Dial()   synched w.  dials many addrs       results to callers
//  ----------------------\    dialsync    use earliest            /--------------
//  -----------------------\              |----------\           /----------------
//  ------------------------>------------<-------     >---------<-----------------
//  -----------------------|              \----x                 \----------------
//  ----------------------|                \-----x                \---------------
//                                         any may fail          if no addr at end
//                                                             retry dialAttempt x

// dialAttempts governs how many times a goroutine will try to dial a given peer.
const dialAttempts = 3

// DialTimeout is the amount of time each dial attempt has. We can think about making
// this larger down the road, or putting more granular timeouts (i.e. within each
// subcomponent of Dial)
var DialTimeout time.Duration = time.Second * 10

// dialsync is a small object that helps manage ongoing dials.
// this way, if we receive many simultaneous dial requests, one
// can do its thing, while the rest wait.
//
// this interface is so would-be dialers can just:
//
//  for {
//  	c := findConnectionToPeer(peer)
//  	if c != nil {
//  		return c
//  	}
//
//  	// ok, no connections. should we dial?
//  	if ok, wait := dialsync.Lock(peer); !ok {
//  		<-wait // can optionally wait
//  		continue
//  	}
//  	defer dialsync.Unlock(peer)
//
//  	c := actuallyDial(peer)
//  	return c
//  }
//
type dialsync struct {
	// ongoing is a map of tickets for the current peers being dialed.
	// this way, we dont kick off N dials simultaneously.
	ongoing map[peer.ID]chan struct{}
	lock    sync.Mutex
}

// Lock governs the beginning of a dial attempt.
// If there are no ongoing dials, it returns true, and the client is now
// scheduled to dial. Every other goroutine that calls startDial -- with
//the same dst -- will block until client is done. The client MUST call
// ds.Unlock(p) when it is done, to unblock the other callers.
// The client is not reponsible for achieving a successful dial, only for
// reporting the end of the attempt (calling ds.Unlock(p)).
//
// see the example below `dialsync`
func (ds *dialsync) Lock(dst peer.ID) (bool, chan struct{}) {
	ds.lock.Lock()
	if ds.ongoing == nil { // init if not ready
		ds.ongoing = make(map[peer.ID]chan struct{})
	}
	wait, found := ds.ongoing[dst]
	if !found {
		ds.ongoing[dst] = make(chan struct{})
	}
	ds.lock.Unlock()

	if found {
		return false, wait
	}

	// ok! you're signed up to dial!
	return true, nil
}

// Unlock releases waiters to a dial attempt. see Lock.
// if Unlock(p) is called without calling Lock(p) first, Unlock panics.
func (ds *dialsync) Unlock(dst peer.ID) {
	ds.lock.Lock()
	wait, found := ds.ongoing[dst]
	if !found {
		panic("called dialDone with no ongoing dials to peer: " + dst.Pretty())
	}
	delete(ds.ongoing, dst) // remove ongoing dial
	close(wait)             // release everyone else
	ds.lock.Unlock()
}

// dialbackoff is a struct used to avoid over-dialing the same, dead peers.
// Whenever we totally time out on a peer (all three attempts), we add them
// to dialbackoff. Then, whenevers goroutines would _wait_ (dialsync), they
// check dialbackoff. If it's there, they don't wait and exit promptly with
// an error. (the single goroutine that is actually dialing continues to
// dial). If a dial is successful, the peer is removed from backoff.
// Example:
//
//  for {
//  	if ok, wait := dialsync.Lock(p); !ok {
//  		if backoff.Backoff(p) {
//  			return errDialFailed
//  		}
//  		<-wait
//  		continue
//  	}
//  	defer dialsync.Unlock(p)
//  	c, err := actuallyDial(p)
//  	if err != nil {
//  		dialbackoff.AddBackoff(p)
//  		continue
//  	}
//  	dialbackoff.Clear(p)
//  }
//
type dialbackoff struct {
	entries map[peer.ID]struct{}
	lock    sync.RWMutex
}

func (db *dialbackoff) init() {
	if db.entries == nil {
		db.entries = make(map[peer.ID]struct{})
	}
}

// Backoff returns whether the client should backoff from dialing
// peeer p
func (db *dialbackoff) Backoff(p peer.ID) bool {
	db.lock.Lock()
	db.init()
	_, found := db.entries[p]
	db.lock.Unlock()
	return found
}

// AddBackoff lets other nodes know that we've entered backoff with
// peer p, so dialers should not wait unnecessarily. We still will
// attempt to dial with one goroutine, in case we get through.
func (db *dialbackoff) AddBackoff(p peer.ID) {
	db.lock.Lock()
	db.init()
	db.entries[p] = struct{}{}
	db.lock.Unlock()
}

// Clear removes a backoff record. Clients should call this after a
// successful Dial.
func (db *dialbackoff) Clear(p peer.ID) {
	db.lock.Lock()
	db.init()
	delete(db.entries, p)
	db.lock.Unlock()
}

// Dial connects to a peer.
//
// The idea is that the client of Swarm does not need to know what network
// the connection will happen over. Swarm can use whichever it choses.
// This allows us to use various transport protocols, do NAT traversal/relay,
// etc. to achive connection.
func (s *Swarm) Dial(ctx context.Context, p peer.ID) (*Conn, error) {
	if p == s.local {
		return nil, errors.New("Attempted connection to self!")
	}

	// this loop is here because dials take time, and we should not be dialing
	// the same peer concurrently (silly waste). Additonally, it's structured
	// to check s.ConnectionsToPeer(p) _first_, and _between_ attempts because we
	// may have received an incoming connection! if so, we no longer must dial.
	//
	// During the dial attempts, we may be doing the dialing. if not, we wait.
	var err error
	var conn *Conn
	for i := 0; i < dialAttempts; i++ {
		// check if we already have an open connection first
		cs := s.ConnectionsToPeer(p)
		for _, conn = range cs {
			if conn != nil { // dump out the first one we find. (TODO pick better)
				return conn, nil
			}
		}

		// check if there's an ongoing dial to this peer
		if ok, wait := s.dsync.Lock(p); !ok {

			if s.backf.Backoff(p) {
				log.Debugf("backoff")
				return nil, fmt.Errorf("%s failed to dial %s, backing off.", s.local, p)
			}

			log.Debugf("waiting for ongoing dial")
			select {
			case <-wait: // wait for that dial to finish.
				continue // and see if it worked (loop), OR we got an incoming dial.
			case <-ctx.Done(): // or we may have to bail...
				return nil, ctx.Err()
			}
		}

		// ok, we have been charged to dial! let's do it.
		// if it succeeds, dial will add the conn to the swarm itself.
		log.Debugf("dial start")
		ctxT, _ := context.WithTimeout(ctx, s.dialT)
		conn, err = s.dial(ctxT, p)
		s.dsync.Unlock(p)
		log.Debugf("dial end %s", conn)
		if err != nil {
			s.backf.AddBackoff(p) // let others know to backoff

			continue // ok, we failed. try again. (if loop is done, our error is output)
		}
		s.backf.Clear(p) // okay, no longer need to backoff
		return conn, nil
	}
	if err == nil {
		err = fmt.Errorf("%s failed to dial %s after %d attempts", s.local, p, dialAttempts)
	}
	return nil, err
}

// dial is the actual swarm's dial logic, gated by Dial.
func (s *Swarm) dial(ctx context.Context, p peer.ID) (*Conn, error) {
	if p == s.local {
		return nil, errors.New("Attempted connection to self!")
	}

	sk := s.peers.PrivKey(s.local)
	if sk == nil {
		// may be fine for sk to be nil, just log a warning.
		log.Warning("Dial not given PrivateKey, so WILL NOT SECURE conn.")
	}

	// get our own addrs. try dialing out from our listener addresses (reusing ports)
	// Note that using our peerstore's addresses here is incorrect, as that would
	// include observed addresses. TODO: make peerstore's address book smarter.
	localAddrs := s.ListenAddresses()
	if len(localAddrs) == 0 {
		log.Debug("Dialing out with no local addresses.")
	}

	// get remote peer addrs
	remoteAddrs := s.peers.Addresses(p)
	// make sure we can use the addresses.
	remoteAddrs = addrutil.FilterUsableAddrs(remoteAddrs)
	// drop out any addrs that would just dial ourselves. use ListenAddresses
	// as that is a more authoritative view than localAddrs.
	ila, _ := s.InterfaceListenAddresses()
	remoteAddrs = addrutil.Subtract(remoteAddrs, ila)
	remoteAddrs = addrutil.Subtract(remoteAddrs, s.peers.Addresses(s.local))
	log.Debugf("%s swarm dialing %s -- remote:%s local:%s", s.local, p, remoteAddrs, s.ListenAddresses())
	if len(remoteAddrs) == 0 {
		return nil, errors.New("peer has no addresses")
	}

	// open connection to peer
	d := &conn.Dialer{
		Dialer: manet.Dialer{
			Dialer: net.Dialer{
				Timeout: s.dialT,
			},
		},
		LocalPeer:  s.local,
		LocalAddrs: localAddrs,
		PrivateKey: sk,
	}

	// try to get a connection to any addr
	connC, err := s.dialAddrs(ctx, d, p, remoteAddrs)
	if err != nil {
		return nil, err
	}

	// ok try to setup the new connection.
	swarmC, err := dialConnSetup(ctx, s, connC)
	if err != nil {
		log.Debug("Dial newConnSetup failed. disconnecting.")
		log.Event(ctx, "dialFailureDisconnect", lgbl.NetConn(connC), lgbl.Error(err))
		connC.Close() // close the connection. didn't work out :(
		return nil, err
	}

	log.Event(ctx, "dial", p)
	return swarmC, nil
}

func (s *Swarm) dialAddrs(ctx context.Context, d *conn.Dialer, p peer.ID, remoteAddrs []ma.Multiaddr) (conn.Conn, error) {

	// try to connect to one of the peer's known addresses.
	// we dial concurrently to each of the addresses, which:
	// * makes the process faster overall
	// * attempts to get the fastest connection available.
	// * mitigates the waste of trying bad addresses
	log.Debugf("%s swarm dialing %s %s", s.local, p, remoteAddrs)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // cancel work when we exit func

	foundConn := make(chan struct{})
	conns := make(chan conn.Conn, len(remoteAddrs))
	errs := make(chan error, len(remoteAddrs))

	//TODO: rate limiting just in case?
	for _, addr := range remoteAddrs {
		go func(addr ma.Multiaddr) {
			connC, err := s.dialAddr(ctx, d, p, addr)

			// check parent still wants our results
			select {
			case <-foundConn:
				if connC != nil {
					connC.Close()
				}
				return
			default:
			}

			if err != nil {
				errs <- err
			} else if connC == nil {
				errs <- fmt.Errorf("failed to dial %s %s", p, addr)
			} else {
				conns <- connC
			}
		}(addr)
	}

	err := fmt.Errorf("failed to dial %s", p)
	for i := 0; i < len(remoteAddrs); i++ {
		select {
		case err = <-errs:
			log.Info(err)
		case connC := <-conns:
			// take the first + return asap
			close(foundConn)
			return connC, nil
		}
	}
	return nil, err
}

func (s *Swarm) dialAddr(ctx context.Context, d *conn.Dialer, p peer.ID, addr ma.Multiaddr) (conn.Conn, error) {
	log.Debugf("%s swarm dialing %s %s", s.local, p, addr)

	connC, err := d.Dial(ctx, addr, p)
	if err != nil {
		return nil, fmt.Errorf("%s --> %s dial attempt failed: %s", s.local, p, err)
	}

	// if the connection is not to whom we thought it would be...
	remotep := connC.RemotePeer()
	if remotep != p {
		connC.Close()
		return nil, fmt.Errorf("misdial to %s through %s (got %s)", p, addr, remotep)
	}

	// if the connection is to ourselves...
	// this can happen TONS when Loopback addrs are advertized.
	// (this should be caught by two checks above, but let's just make sure.)
	if remotep == s.local {
		connC.Close()
		return nil, fmt.Errorf("misdial to %s through %s (got self)", p, addr)
	}

	// success! we got one!
	return connC, nil
}

// dialConnSetup is the setup logic for a connection from the dial side. it
// needs to add the Conn to the StreamSwarm, then run newConnSetup
func dialConnSetup(ctx context.Context, s *Swarm, connC conn.Conn) (*Conn, error) {

	psC, err := s.swarm.AddConn(connC)
	if err != nil {
		// connC is closed by caller if we fail.
		return nil, fmt.Errorf("failed to add conn to ps.Swarm: %s", err)
	}

	// ok try to setup the new connection. (newConnSetup will add to group)
	swarmC, err := s.newConnSetup(ctx, psC)
	if err != nil {
		log.Debug("Dial newConnSetup failed. disconnecting.")
		log.Event(ctx, "dialFailureDisconnect", lgbl.NetConn(connC), lgbl.Error(err))
		psC.Close() // we need to make sure psC is Closed.
		return nil, err
	}

	return swarmC, err
}
