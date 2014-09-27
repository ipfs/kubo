package swarm

import (
	"errors"
	"fmt"
	"net"
	"sync"

	conn "github.com/jbenet/go-ipfs/net/conn"
	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

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
type Swarm struct {

	// local is the peer this swarm represents
	local *peer.Peer

	// peers is a collection of peers for swarm to use
	peers peer.Peerstore

	// Swarm includes a Pipe object.
	*msg.Pipe

	// errChan is the channel of errors.
	errChan chan error

	// conns are the open connections the swarm is handling.
	conns     conn.Map
	connsLock sync.RWMutex

	// listeners for each network address
	listeners []net.Listener

	// cancel is an internal function used to stop the Swarm's processing.
	cancel context.CancelFunc
	ctx    context.Context
}

// NewSwarm constructs a Swarm, with a Chan.
func NewSwarm(ctx context.Context, local *peer.Peer, ps peer.Peerstore) (*Swarm, error) {
	s := &Swarm{
		Pipe:    msg.NewPipe(10),
		conns:   conn.Map{},
		local:   local,
		peers:   ps,
		errChan: make(chan error, 100),
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	go s.fanOut()
	return s, s.listen()
}

// Close stops a swarm.
func (s *Swarm) Close() error {
	if s.cancel == nil {
		return errors.New("Swarm already closed.")
	}

	// issue cancel for the context
	s.cancel()

	// set cancel to nil to prevent calling Close again, and signal to Listeners
	s.cancel = nil

	// close listeners
	for _, list := range s.listeners {
		list.Close()
	}
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
func (s *Swarm) Dial(peer *peer.Peer) (*conn.Conn, error) {
	if peer.ID.Equal(s.local.ID) {
		return nil, errors.New("Attempted connection to self!")
	}

	// check if we already have an open connection first
	c := s.GetConnection(peer.ID)
	if c != nil {
		return c, nil
	}

	// check if we don't have the peer in Peerstore
	err := s.peers.Put(peer)
	if err != nil {
		return nil, err
	}

	// open connection to peer
	c, err = conn.Dial("tcp", peer)
	if err != nil {
		return nil, err
	}

	if err := s.connSetup(c); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

// DialAddr is for connecting to a peer when you know their addr but not their ID.
// Should only be used when sure that not connected to peer in question
// TODO(jbenet) merge with Dial? need way to patch back.
func (s *Swarm) DialAddr(addr *ma.Multiaddr) (*conn.Conn, error) {
	if addr == nil {
		return nil, errors.New("addr must be a non-nil Multiaddr")
	}

	npeer := new(peer.Peer)
	npeer.AddAddress(addr)

	c, err := conn.Dial("tcp", npeer)
	if err != nil {
		return nil, err
	}

	if err := s.connSetup(c); err != nil {
		c.Close()
		return nil, err
	}

	return c, err
}

// GetConnection returns the connection in the swarm to given peer.ID
func (s *Swarm) GetConnection(pid peer.ID) *conn.Conn {
	s.connsLock.RLock()
	c, found := s.conns[u.Key(pid)]
	s.connsLock.RUnlock()

	if !found {
		return nil
	}
	return c
}

// CloseConnection removes a given peer from swarm + closes the connection
func (s *Swarm) CloseConnection(p *peer.Peer) error {
	c := s.GetConnection(p.ID)
	if c == nil {
		return u.ErrNotFound
	}

	s.connsLock.Lock()
	delete(s.conns, u.Key(p.ID))
	s.connsLock.Unlock()

	return c.Close()
}

func (s *Swarm) Error(e error) {
	s.errChan <- e
}

// GetErrChan returns the errors chan.
func (s *Swarm) GetErrChan() chan error {
	return s.errChan
}

// Temporary to ensure that the Swarm always matches the Network interface as we are changing it
// var _ Network = &Swarm{}
