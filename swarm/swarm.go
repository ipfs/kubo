package swarm

import (
	"fmt"
	peer "github.com/jbenet/go-ipfs/peer"
	"sync"
)

// Message represents a packet of information sent to or received from a
// particular Peer.
type Message struct {
	// To or from, depending on direction.
	Peer *peer.Peer

	// Opaque data
	Data []byte
}

// Chan is a swam channel, which provides duplex communication and errors.
type Chan struct {
	Outgoing chan Message
	Incoming chan Message
	Errors   chan error
	Close    chan bool
}

// NewChan constructs a Chan instance, with given buffer size bufsize.
func NewChan(bufsize int) *Chan {
	return &Chan{
		Outgoing: make(chan Message, bufsize),
		Incoming: make(chan Message, bufsize),
		Errors:   make(chan error),
		Close:    make(chan bool, bufsize),
	}
}

// Swarm is a connection muxer, allowing connections to other peers to
// be opened and closed, while still using the same Chan for all
// communication. The Chan sends/receives Messages, which note the
// destination or source Peer.
type Swarm struct {
	Chan      *Chan
	conns     ConnMap
	connsLock sync.RWMutex
}

// NewSwarm constructs a Swarm, with a Chan.
func NewSwarm() *Swarm {
	s := &Swarm{
		Chan:  NewChan(10),
		conns: ConnMap{},
	}
	go s.fanOut()
	return s
}

// Close closes a swam.
func (s *Swarm) Close() {
	s.connsLock.RLock()
	l := len(s.conns)
	s.connsLock.RUnlock()

	for i := 0; i < l; i++ {
		s.Chan.Close <- true // fan ins
	}
	s.Chan.Close <- true // fan out
	s.Chan.Close <- true // listener
}

// Dial connects to a peer.
//
// The idea is that the client of Swarm does not need to know what network
// the connection will happen over. Swarm can use whichever it choses.
// This allows us to use various transport protocols, do NAT traversal/relay,
// etc. to achive connection.
//
// For now, Dial uses only TCP. This will be extended.
func (s *Swarm) Dial(peer *peer.Peer) (*Conn, error) {
	k := peer.Key()

	// check if we already have an open connection first
	s.connsLock.RLock()
	conn, found := s.conns[k]
	s.connsLock.RUnlock()
	if found {
		return conn, nil
	}

	// open connection to peer
	conn, err := Dial("tcp", peer)
	if err != nil {
		return nil, err
	}

	// add to conns
	s.connsLock.Lock()
	s.conns[k] = conn
	s.connsLock.Unlock()

	// kick off reader goroutine
	go s.fanIn(conn)
	return conn, nil
}

// Handles the unwrapping + sending of messages to the right connection.
func (s *Swarm) fanOut() {
	for {
		select {
		case <-s.Chan.Close:
			return // told to close.
		case msg, ok := <-s.Chan.Outgoing:
			if !ok {
				return
			}

			s.connsLock.RLock()
			conn, found := s.conns[msg.Peer.Key()]
			s.connsLock.RUnlock()
			if !found {
				e := fmt.Errorf("Sent msg to peer without open conn: %v", msg.Peer)
				s.Chan.Errors <- e
			}

			// queue it in the connection's buffer
			conn.Outgoing.MsgChan <- msg.Data
		}
	}
}

// Handles the receiving + wrapping of messages, per conn.
// Consider using reflect.Select with one goroutine instead of n.
func (s *Swarm) fanIn(conn *Conn) {
Loop:
	for {
		select {
		case <-s.Chan.Close:
			// close Conn.
			conn.Close()
			break Loop

		case <-conn.Closed:
			break Loop

		case data, ok := <-conn.Incoming.MsgChan:
			if !ok {
				e := fmt.Errorf("Error retrieving from conn: %v", conn)
				s.Chan.Errors <- e
				break Loop
			}

			// wrap it for consumers.
			msg := Message{Peer: conn.Peer, Data: data}
			s.Chan.Incoming <- msg
		}
	}

	s.connsLock.Lock()
	delete(s.conns, conn.Peer.Key())
	s.connsLock.Unlock()
}
