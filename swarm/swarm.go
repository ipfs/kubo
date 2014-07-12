package swarm

import (
	"fmt"
	peer "github.com/jbenet/go-ipfs/peer"
	"sync"
)

type Message struct {
	// To or from, depending on direction.
	Peer *peer.Peer

	// Opaque data
	Data []byte
}

type Chan struct {
	Outgoing chan Message
	Incoming chan Message
	Errors   chan error
	Close    chan bool
}

func NewChan(bufsize int) *Chan {
	return &Chan{
		Outgoing: make(chan Message, bufsize),
		Incoming: make(chan Message, bufsize),
		Errors:   make(chan error),
		Close:    make(chan bool, bufsize),
	}
}

type Swarm struct {
	Chan      *Chan
	conns     ConnMap
	connsLock sync.RWMutex
}

func NewSwarm() *Swarm {
	s := &Swarm{
		Chan:  NewChan(10),
		conns: ConnMap{},
	}
	go s.fanOut()
	return s
}

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
			fmt.Println("got back data", data)
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
