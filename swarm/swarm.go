package swarm

import (
	"errors"
	"fmt"
	"net"
	"sync"

	proto "code.google.com/p/goprotobuf/proto"
	ident "github.com/jbenet/go-ipfs/identify"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	ma "github.com/jbenet/go-multiaddr"
)

var ErrAlreadyOpen = errors.New("Error: Connection to this peer already open.")

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
	bytes, err := proto.Marshal(data)
	if err != nil {
		u.PErr("%v\n", err.Error())
		return nil
	}
	return &Message{
		Peer: p,
		Data: bytes,
	}
}

// Chan is a swarm channel, which provides duplex communication and errors.
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
		Errors:   make(chan error, bufsize),
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
	for i, v := range se.Errors {
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

	filterChans map[PBWrapper_MessageType]*Chan
	toFilter    chan *Message
	newFilters  chan *newFilterInfo

	local     *peer.Peer
	listeners []net.Listener
	haltroute chan struct{}
}

// NewSwarm constructs a Swarm, with a Chan.
func NewSwarm(local *peer.Peer) *Swarm {
	s := &Swarm{
		Chan:        NewChan(10),
		conns:       ConnMap{},
		local:       local,
		filterChans: make(map[PBWrapper_MessageType]*Chan),
		toFilter:    make(chan *Message, 32),
		newFilters:  make(chan *newFilterInfo),
		haltroute:   make(chan struct{}),
	}
	go s.routeMessages()
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
				go func() { s.Chan.Errors <- e }()
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
		u.PErr("%v\n", err.Error())
		conn.Close()
		return
	}

	// Get address to contact remote peer from
	addr := <-conn.Incoming.MsgChan
	maddr, err := ma.NewMultiaddr(string(addr))
	if err != nil {
		u.PErr("Got invalid address from peer.")
	}
	p.AddAddress(maddr)

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

	for _, list := range s.listeners {
		list.Close()
	}

	s.haltroute <- struct{}{}

	for _, filter := range s.filterChans {
		filter.Close <- true
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
func (s *Swarm) Dial(peer *peer.Peer) (*Conn, error, bool) {
	k := peer.Key()

	// check if we already have an open connection first
	s.connsLock.RLock()
	conn, found := s.conns[k]
	s.connsLock.RUnlock()
	if found {
		return conn, nil, true
	}

	// open connection to peer
	conn, err := Dial("tcp", peer)
	if err != nil {
		return nil, err, false
	}

	return conn, nil, false
}

// StartConn adds the passed in connection to its peerMap and starts
// the fanIn routine for that connection
func (s *Swarm) StartConn(conn *Conn) error {
	if conn == nil {
		return errors.New("Tried to start nil connection.")
	}

	u.DOut("Starting connection: %s\n", conn.Peer.Key().Pretty())
	// add to conns
	s.connsLock.Lock()
	if _, ok := s.conns[conn.Peer.Key()]; ok {
		s.connsLock.Unlock()
		return ErrAlreadyOpen
	}
	s.conns[conn.Peer.Key()] = conn
	s.connsLock.Unlock()

	// kick off reader goroutine
	go s.fanIn(conn)
	return nil
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
				e := fmt.Errorf("Sent msg to peer without open conn: %v",
					msg.Peer)
				s.Chan.Errors <- e
				continue
			}

			// queue it in the connection's buffer
			conn.Outgoing.MsgChan <- msg.Data
		}
	}
}

// Handles the receiving + wrapping of messages, per conn.
// Consider using reflect.Select with one goroutine instead of n.
func (s *Swarm) fanIn(conn *Conn) {
	for {
		select {
		case <-s.Chan.Close:
			// close Conn.
			conn.Close()
			goto out

		case <-conn.Closed:
			goto out

		case data, ok := <-conn.Incoming.MsgChan:
			if !ok {
				e := fmt.Errorf("Error retrieving from conn: %v", conn.Peer.Key().Pretty())
				s.Chan.Errors <- e
				goto out
			}

			msg := &Message{Peer: conn.Peer, Data: data}
			s.toFilter <- msg
		}
	}
out:

	s.connsLock.Lock()
	delete(s.conns, conn.Peer.Key())
	s.connsLock.Unlock()
}

type newFilterInfo struct {
	Type PBWrapper_MessageType
	resp chan *Chan
}

func (s *Swarm) routeMessages() {
	for {
		select {
		case mes, ok := <-s.toFilter:
			if !ok {
				return
			}
			wrapper, err := Unwrap(mes.Data)
			if err != nil {
				u.PErr("error in route messages: %s\n", err)
			}

			ch, ok := s.filterChans[PBWrapper_MessageType(wrapper.GetType())]
			if !ok {
				u.PErr("Received message with invalid type: %d\n", wrapper.GetType())
				continue
			}

			mes.Data = wrapper.GetMessage()
			ch.Incoming <- mes
		case gchan := <-s.newFilters:
			nch, ok := s.filterChans[gchan.Type]
			if !ok {
				nch = NewChan(16)
				s.filterChans[gchan.Type] = nch
				go s.muxChan(nch, gchan.Type)
			}
			gchan.resp <- nch
		case <-s.haltroute:
			return
		}
	}
}

func (s *Swarm) muxChan(ch *Chan, typ PBWrapper_MessageType) {
	for {
		select {
		case <-ch.Close:
			return
		case mes := <-ch.Outgoing:
			data, err := Wrap(mes.Data, typ)
			if err != nil {
				u.PErr("muxChan error: %s\n", err)
				continue
			}
			mes.Data = data
			s.Chan.Outgoing <- mes
		}
	}
}

func (s *Swarm) Find(key u.Key) *peer.Peer {
	s.connsLock.RLock()
	defer s.connsLock.RUnlock()
	conn, found := s.conns[key]
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

	if id.Equal(s.local.ID) {
		panic("Attempted connection to self!")
	}

	conn, err, reused := s.Dial(p)
	if err != nil {
		return nil, err
	}

	if reused {
		return p, nil
	}

	err = s.handleDialedCon(conn)
	return conn.Peer, err
}

// Handle performing a handshake on a new connection and ensuring proper forward communication
func (s *Swarm) handleDialedCon(conn *Conn) error {
	err := ident.Handshake(s.local, conn.Peer, conn.Incoming.MsgChan, conn.Outgoing.MsgChan)
	if err != nil {
		return err
	}

	// Send node an address that you can be reached on
	myaddr := s.local.NetAddress("tcp")
	mastr, err := myaddr.String()
	if err != nil {
		errors.New("No local address to send to peer.")
	}

	conn.Outgoing.MsgChan <- []byte(mastr)

	s.StartConn(conn)

	return nil
}

// ConnectNew is for connecting to a peer when you dont know their ID,
// Should only be used when you are sure that you arent already connected to peer in question
func (s *Swarm) ConnectNew(addr *ma.Multiaddr) (*peer.Peer, error) {
	if addr == nil {
		return nil, errors.New("nil Multiaddr passed to swarm.Connect()")
	}
	npeer := new(peer.Peer)
	npeer.AddAddress(addr)

	conn, err := Dial("tcp", npeer)
	if err != nil {
		return nil, err
	}

	err = s.handleDialedCon(conn)
	return npeer, err
}

// Removes a given peer from the swarm and closes connections to it
func (s *Swarm) Drop(p *peer.Peer) error {
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
	s.Chan.Errors <- e
}

func (s *Swarm) GetErrChan() chan error {
	return s.Chan.Errors
}

func (s *Swarm) GetChannel(typ PBWrapper_MessageType) *Chan {
	nfi := &newFilterInfo{
		Type: typ,
		resp: make(chan *Chan),
	}
	s.newFilters <- nfi

	return <-nfi.resp
}

// Temporary to ensure that the Swarm always matches the Network interface as we are changing it
var _ Network = &Swarm{}
