package swarm

import (
	"fmt"
	"net"
	"sync"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	ma "github.com/jbenet/go-multiaddr"
	ident "github.com/jbenet/go-ipfs/identify"
	proto "code.google.com/p/goprotobuf/proto"
)

// Message represents a packet of information sent to or received from a
// particular Peer.
type Message struct {
	// To or from, depending on direction.
	Peer *peer.Peer

	// Opaque data
	Data []byte
}

// Cleaner looking helper function to make a new message struct
func NewMessage(p *peer.Peer, data proto.Message) *Message {
	bytes,err := proto.Marshal(data)
	if err != nil {
		panic(err)
	}
	return &Message{
		Peer: p,
		Data: bytes,
	}
}

// Chan is a swam channel, which provides duplex communication and errors.
type Chan struct {
	Outgoing chan *Message
	Incoming chan *Message
	Errors   chan error
	Close    chan bool
}

// NewChan constructs a Chan instance, with given buffer size bufsize.
func NewChan(bufsize int) *Chan {
	return &Chan{
		Outgoing: make(chan *Message, bufsize),
		Incoming: make(chan *Message, bufsize),
		Errors:   make(chan error),
		Close:    make(chan bool, bufsize),
	}
}

// Contains a set of errors mapping to each of the swarms addresses
// that were listened on
type SwarmListenErr struct {
	Errors []error
}

func (se *SwarmListenErr) Error() string {
	if se == nil {
		return "<nil error>"
	}
	var out string
	for i,v := range se.Errors {
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
	Chan      *Chan
	conns     ConnMap
	connsLock sync.RWMutex

	local *peer.Peer
	listeners []net.Listener
}

// NewSwarm constructs a Swarm, with a Chan.
func NewSwarm(local *peer.Peer) *Swarm {
	s := &Swarm{
		Chan:  NewChan(10),
		conns: ConnMap{},
		local: local,
	}
	go s.fanOut()
	return s
}

// Open listeners for each network the swarm should listen on
func (s *Swarm) Listen() error {
	var ret_err *SwarmListenErr
	for i, addr := range s.local.Addresses {
		err := s.connListen(addr)
		if err != nil {
			if ret_err == nil {
				ret_err = new(SwarmListenErr)
				ret_err.Errors = make([]error, len(s.local.Addresses))
			}
			ret_err.Errors[i] = err
			u.PErr("Failed to listen on: %s [%s]", addr, err)
		}
	}
	if ret_err == nil {
		return nil
	}
	return ret_err
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
				s.Chan.Errors <- e
				return
			}
			go s.handleNewConn(nconn)
		}
	}()

	return nil
}

// Handle getting ID from this peer and adding it into the map
func (s *Swarm) handleNewConn(nconn net.Conn) {
	p := new(peer.Peer)

	conn := &Conn{
		Peer: p,
		Addr: nil,
		Conn: nconn,
	}
	newConnChans(conn)

	err := ident.Handshake(s.local, p, conn.Incoming.MsgChan, conn.Outgoing.MsgChan)
	if err != nil {
		panic(err)
	}

	s.StartConn(conn)
}

// Close closes a swarm.
func (s *Swarm) Close() {
	s.connsLock.RLock()
	l := len(s.conns)
	s.connsLock.RUnlock()

	for i := 0; i < l; i++ {
		s.Chan.Close <- true // fan ins
	}
	s.Chan.Close <- true // fan out
	s.Chan.Close <- true // listener

	for _,list := range s.listeners {
		list.Close()
	}
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

	s.StartConn(conn)
	return conn, nil
}

func (s *Swarm) StartConn(conn *Conn) {
	if conn == nil {
		panic("tried to start nil Conn!")
	}

	u.DOut("Starting connection: %s", string(conn.Peer.ID))
	// add to conns
	s.connsLock.Lock()
	s.conns[conn.Peer.Key()] = conn
	s.connsLock.Unlock()

	// kick off reader goroutine
	go s.fanIn(conn)
}

// Handles the unwrapping + sending of messages to the right connection.
func (s *Swarm) fanOut() {
	for {
		select {
		case <-s.Chan.Close:
			return // told to close.
		case msg, ok := <-s.Chan.Outgoing:
			u.DOut("fanOut: outgoing message for: '%s'", msg.Peer.Key())
			if !ok {
				return
			}

			s.connsLock.RLock()
			conn, found := s.conns[msg.Peer.Key()]
			s.connsLock.RUnlock()

			if !found {
				e := fmt.Errorf("Sent msg to peer without open conn: %v",
					msg.Peer)
				s.Chan.Errors <- e
				continue
			}

			// queue it in the connection's buffer
			conn.Outgoing.MsgChan <- msg.Data
			u.DOut("fanOut: message off.")
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
			u.DOut("fanIn: got message from incoming channel.")
			if !ok {
				e := fmt.Errorf("Error retrieving from conn: %v", conn)
				s.Chan.Errors <- e
				break Loop
			}

			// wrap it for consumers.
			msg := &Message{Peer: conn.Peer, Data: data}
			s.Chan.Incoming <- msg
			u.DOut("fanIn: message off.")
		}
	}

	s.connsLock.Lock()
	delete(s.conns, conn.Peer.Key())
	s.connsLock.Unlock()
}

func (s *Swarm) Find(addr *ma.Multiaddr) {
}
