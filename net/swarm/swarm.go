// package swarm implements a connection muxer with a pair of channels
// to synchronize all network communication.
package swarm

import (
	"errors"
	"fmt"

	conn "github.com/jbenet/go-ipfs/net/conn"
	peer "github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/util/eventlog"

	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	router "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-router"
)

var log = eventlog.Logger("swarm")

// ErrAlreadyOpen signals that a connection to a peer is already open.
var ErrAlreadyOpen = errors.New("Error: Connection to this peer already open.")

// ListenErr contains a set of errors mapping to each of the swarms addresses.
// Used to return multiple errors, as in listen.
type ListenErr struct {
	Errors []error
}

func (e *ListenErr) Error() string {
	if e == nil {
		return "<nil error>"
	}
	var out string
	for i, v := range e.Errors {
		if v != nil {
			out += fmt.Sprintf("%d: %s\n", i, v)
		}
	}
	return out
}

// Swarm is a connection muxer, allowing connections to other peers to
// be opened and closed, while still using the same Chan for all
// communication. The Chan sends/receives Messages, which note the
// destination or source Peer.
//
// Implements router.Node
type Swarm struct {

	// local is the peer this swarm represents
	local peer.Peer

	// peers is a collection of peers for swarm to use
	peers peer.Peerstore

	// rt handles the open connections the swarm is handling.
	rt *swarmRoutingTable

	// listeners for each network address
	listeners []conn.Listener

	// ContextGroup
	cg ctxgroup.ContextGroup
}

// NewSwarm constructs a Swarm, with a Chan.
func NewSwarm(ctx context.Context, listenAddrs []ma.Multiaddr,
	local peer.Peer, ps peer.Peerstore, client router.Node) (*Swarm, error) {

	s := &Swarm{
		local: local,
		peers: ps,
		cg:    ctxgroup.WithContext(ctx),
		rt:    newRoutingTable(local, client),
	}

	s.cg.SetTeardown(s.close)
	return s, s.listen(listenAddrs)
}

// SetClient assign's the Swarm's client node.
func (s *Swarm) SetClient(n router.Node) {
	s.rt.client = n
}

// Close stops a swarm. waits till it exits
func (s *Swarm) Close() error {
	return s.cg.Close()
}

// close stops a swarm. It's the underlying function called by ContextGroup
func (s *Swarm) close() error {
	// close listeners
	for _, list := range s.listeners {
		list.Close()
	}
	// close connections
	conn.CloseConns(s.Connections()...)
	return nil
}

// Dial connects to a peer.
//
// The idea is that the client of Swarm does not need to know what network
// the connection will happen over. Swarm can use whichever it choses.
// This allows us to use various transport protocols, do NAT traversal/relay,
// etc. to achive connection.
//
// For now, Dial uses only TCP. This will be extended.
func (s *Swarm) Dial(peer peer.Peer) (conn.Conn, error) {
	if peer.ID().Equal(s.local.ID()) {
		return nil, errors.New("Attempted connection to self!")
	}

	// check if we already have an open connection first
	c := s.GetConnection(peer.ID())
	if c != nil {
		return c, nil
	}

	// check if we don't have the peer in Peerstore
	peer, err := s.peers.Add(peer)
	if err != nil {
		return nil, err
	}

	// open connection to peer
	d := &conn.Dialer{
		LocalPeer: s.local,
		Peerstore: s.peers,
	}

	if len(peer.Addresses()) == 0 {
		return nil, errors.New("peer has no addresses")
	}
	// try to connect to one of the peer's known addresses.
	// for simplicity, we do this sequentially.
	// A future commit will do this asynchronously.
	for _, addr := range peer.Addresses() {
		c, err = d.DialAddr(s.cg.Context(), addr, peer)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	c2, err := s.connSetup(context.TODO(), c)
	if err != nil {
		c.Close()
		return nil, err
	}

	// TODO replace the TODO ctx with a context passed in from caller
	log.Event(context.TODO(), "dial", peer)
	return c2, nil
}

// GetConnection returns the connection in the swarm to given peer.ID
func (s *Swarm) GetConnection(pid peer.ID) conn.Conn {
	sp := s.rt.getByID(pid)
	if sp == nil {
		return nil
	}
	return sp.conn
}

// Connections returns a slice of all connections.
func (s *Swarm) Connections() []conn.Conn {
	return s.rt.connList()
}

// CloseConnection removes a given peer from swarm + closes the connection
func (s *Swarm) CloseConnection(p peer.Peer) error {
	return s.closeConn(p)
}

// GetPeerList returns a copy of the set of peers swarm is connected to.
func (s *Swarm) GetPeerList() []peer.Peer {
	return s.rt.peerList()
}

// LocalPeer returns the local peer swarm is associated to.
func (s *Swarm) LocalPeer() peer.Peer {
	return s.local
}

// Address returns the address of *this* service.
func (s *Swarm) Address() router.Address {
	return "/ipfs/service/swarm" // for now dont need anything more complicated.
}
