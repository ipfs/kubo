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
func NewSwarm(ctx context.Context, local *peer.Peer) (*Swarm, error) {
	s := &Swarm{
		Pipe:    msg.NewPipe(10),
		conns:   conn.Map{},
		local:   local,
		errChan: make(chan error, 100),
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	go s.fanOut()
	return s, s.listen()
}

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
			u.PErr("Failed to listen on: %s [%s]", addr, err)
		}
	}

	if hasErr {
		return retErr
	}
	return nil
}

// Listen for new connections on the given multiaddr
func (s *Swarm) connListen(maddr *ma.Multiaddr) error {
	netstr, addr, err := maddr.DialArgs()
	if err != nil {
		return err
	}

	list, err := net.Listen(netstr, addr)
	if err != nil {
		return err
	}

	// NOTE: this may require a lock around it later. currently, only run on setup
	s.listeners = append(s.listeners, list)

	// Accept and handle new connections on this listener until it errors
	go func() {
		for {
			nconn, err := list.Accept()
			if err != nil {
				e := fmt.Errorf("Failed to accept connection: %s - %s [%s]",
					netstr, addr, err)
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

	k := peer.Key()

	// check if we already have an open connection first
	s.connsLock.RLock()
	c, found := s.conns[k]
	s.connsLock.RUnlock()
	if found {
		return c, nil
	}

	// open connection to peer
	c, err := conn.Dial("tcp", peer)
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
			conn, found := s.conns[msg.Peer.Key()]
			s.connsLock.RUnlock()

			if !found {
				e := fmt.Errorf("Sent msg to peer without open conn: %v",
					msg.Peer)
				s.errChan <- e
				continue
			}

			// queue it in the connection's buffer
			conn.Outgoing.MsgChan <- msg.Data
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

		case data, ok := <-c.Incoming.MsgChan:
			if !ok {
				e := fmt.Errorf("Error retrieving from conn: %v", c.Peer.Key().Pretty())
				s.errChan <- e
				goto out
			}

			msg := &msg.Message{Peer: c.Peer, Data: data}
			s.Incoming <- msg
		}
	}

out:
	s.connsLock.Lock()
	delete(s.conns, c.Peer.Key())
	s.connsLock.Unlock()
}

// GetPeer returns the peer in the swarm with given key id.
func (s *Swarm) GetPeer(key u.Key) *peer.Peer {
	s.connsLock.RLock()
	conn, found := s.conns[key]
	s.connsLock.RUnlock()

	if !found {
		return nil
	}
	return conn.Peer
}

// GetConnection will check if we are already connected to the peer in question
// and only open a new connection if we arent already
func (s *Swarm) GetConnection(id peer.ID, addr *ma.Multiaddr) (*peer.Peer, error) {
	p := &peer.Peer{
		ID:        id,
		Addresses: []*ma.Multiaddr{addr},
	}

	c, err := s.Dial(p)
	if err != nil {
		return nil, err
	}

	return c.Peer, nil
}

// CloseConnection removes a given peer from swarm + closes the connection
func (s *Swarm) CloseConnection(p *peer.Peer) error {
	s.connsLock.RLock()
	conn, found := s.conns[u.Key(p.ID)]
	s.connsLock.RUnlock()
	if !found {
		return u.ErrNotFound
	}

	s.connsLock.Lock()
	delete(s.conns, u.Key(p.ID))
	s.connsLock.Unlock()

	return conn.Close()
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
