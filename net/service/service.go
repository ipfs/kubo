package service

import (
	"errors"
	"fmt"
	"sync"

	msg "github.com/jbenet/go-ipfs/net/message"
	u "github.com/jbenet/go-ipfs/util"

	ctxgroup "github.com/jbenet/go-ctxgroup"
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	router "github.com/jbenet/go-router"
)

var log = u.Logger("service")

// ErrNoResponse is returned by Service when a Request did not get a response,
// and no other error happened
var ErrNoResponse = errors.New("no response to request")

// Handler is an interface that objects must implement in order to handle
// a service's requests.
type Handler interface {

	// HandleMessage receives an incoming message, and potentially returns
	// a response message to send back.
	HandleMessage(context.Context, msg.NetMessage) msg.NetMessage
}

// Sender interface for network services.
type Sender interface {
	// SendMessage sends out a given message, without expecting a response.
	SendMessage(ctx context.Context, m msg.NetMessage) error

	// SendRequest sends out a given message, and awaits a response.
	// Set Deadlines or cancellations in the context.Context you pass in.
	SendRequest(ctx context.Context, m msg.NetMessage) (msg.NetMessage, error)
}

// Service is an interface for a net resource with both outgoing (sender) and
// incomig (SetHandler) requests.
type Service interface {
	Sender // can use it to send out msgs
	router.Node // it is a Node in the net topology.

	// SetUplink assigns the Node to send packets out
	SetUplink(router.Node)
	Uplink() router.Node

	// SetHandler assigns the request Handler for this service.
	SetHandler(Handler)
	GetHandler() Handler
}

// Service is a networking component that protocols can use to multiplex
// messages over the same channel, and to issue + handle requests.
type service struct {
	// Handler is the object registered to handle incoming requests.
	Handler     Handler
	HandlerLock sync.RWMutex

	// Requests are all the pending requests on this service.
	Requests     RequestMap
	RequestsLock sync.RWMutex

	// the connection to the outside world
	uplink     router.Node
	uplinkLock sync.RWMutex
	addr router.Address
}

// NewService creates a service object with given type ID and Handler
func NewService(addr router.Address, uplink router.Node, h Handler) Service {
	s := &service{
		Handler:  h,
		Requests: RequestMap{},
		uplink:   uplink,
		addr:     addr,
	}
	return s
}

// sendMessage sends a message out (actual leg work. SendMessage is to export w/o rid)
func (s *service) sendMessage(ctx context.Context, m msg.NetMessage, rid RequestID) error {

	// serialize ServiceMessage wrapper
	data, err := wrapData(m.Data(), rid)
	if err != nil {
		return err
	}

	// log.Debug("Service send message [to = %s]", m.Peer())

	// send message
	m2 := msg.New(m.Peer(), data)

	pkt := msg.Packet{
		Src:
	}

	select {
	case s.Outgoing <- m2:
	case <-ctx.Done():
		return ctx.Err()
	}

	pkt := msg.Packet{
		Src: m.
	}

	return nil
}

// SendMessage sends a message out
func (s *service) SendMessage(ctx context.Context, m msg.NetMessage) error {
	return s.sendMessage(ctx, m, nil)
}

// SendRequest sends a request message out and awaits a response.
func (s *service) SendRequest(ctx context.Context, m msg.NetMessage) (msg.NetMessage, error) {

	// check if we should bail given our contexts
	select {
	default:
	case <-s.Closing():
		return nil, fmt.Errorf("service closed: %s", s.Context().Err())
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// create a request
	r, err := NewRequest(m.Peer().ID())
	if err != nil {
		return nil, err
	}

	// register Request
	s.RequestsLock.Lock()
	s.Requests[r.Key()] = r
	s.RequestsLock.Unlock()

	// defer deleting this request
	defer func() {
		s.RequestsLock.Lock()
		delete(s.Requests, r.Key())
		s.RequestsLock.Unlock()
	}()

	// check if we should bail after waiting for mutex
	select {
	default:
	case <-s.Closing():
		return nil, fmt.Errorf("service closed: %s", s.Context().Err())
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Send message
	s.sendMessage(ctx, m, r.ID)

	// wait for response
	m = nil
	err = nil
	select {
	case m = <-r.Response:
	case <-s.Closed():
		err = fmt.Errorf("service closed: %s", s.Context().Err())
	case <-ctx.Done():
		err = ctx.Err()
	}

	if m == nil {
		return nil, ErrNoResponse
	}

	return m, err
}

// handleIncoming consumes the messages on the s.Incoming channel and
// routes them appropriately (to requests, or handler).
func (s *service) handleIncomingMessages() {
	defer s.Children().Done()

	for {
		select {
		case m, more := <-s.Incoming:
			if !more {
				return
			}
			s.Children().Add(1)
			go s.handleIncomingMessage(m)

		case <-s.Closing():
			return
		}
	}
}

func (s *service) handleIncomingMessage(pkt *msg.Packet) error {

	// check the packet has a valid Context
	ctx := pkt.Context
	if ctx == nil {
		return fmt.Errorf("service got pkt without valid Context")
	}

	// check the source is a peer
	srcPeer, ok := pkt.Src.(peer.Peer)
	if !ok {
		return fmt.Errorf("service got pkt from non-Peer src: %v", pkt.Src)
	}

	// unwrap the incoming message
	data, rid, err := unwrapData(pkt.Data)
	if err != nil {
		return fmt.Errorf("service de-serializing error: %v", err)
	}

	// convert to msg.NetMessage, which the rest of the system expects.
	m2 := msg.New(srcPeer, data)

	// if it's a request (or has no RequestID), handle it
	if rid == nil || rid.IsRequest() {
		handler := s.GetHandler()
		if handler == nil {
			log.Errorf("service dropped msg: %v", m)
			log.Event()
			return nil
			// no handler, drop it.
		}

		// this go routine is developer friendliness to keep their stacks
		// separate (and more readable) from the network goroutine. If
		// problems arise and you'd like to see _the full_ stack of where
		// this message is coming from, just remove the goroutine part.
		response := make(chan msg.NetMessage)
		go func() msg.NetMessage {
			return handler.HandleMessage(ctx, m2)
		}()
		r1 := <-response
		// Note: HandleMessage *must* respect context. We could co-opt it
		// and do a select {} here on the context, BUT that would just drop
		// a packet and free up the goroutine to return to the network. the
		// problem is still there: the Service handler hasn't returned yet.

		// if handler gave us a response, send it out!
		if r1 != nil {
			if err := s.sendMessage(ctx, r1, rid.Response()); err != nil {
				return fmt.Errorf("error sending response message: %v", err)
			}
		}
		return
	}

	// Otherwise, it is a response. handle it.
	if !rid.IsResponse() {
		log.Errorf("RequestID should identify a response here.")
	}

	key := RequestKey(m.Peer().ID(), RequestID(rid))
	s.RequestsLock.RLock()
	r, found := s.Requests[key]
	s.RequestsLock.RUnlock()

	if !found {
		log.Errorf("no request key %v (timeout?)", []byte(key))
		return
	}

	select {
	case r.Response <- m2:
	case <-s.Closing():
	}
}


// Address is the router.Node address
func (s *service) Address() router.Address {
	return s.addr
}

// HandlePacket implements router.Node
// service only receives packets in HandlePacket
func (s *service) HandlePacket(p router.Packet, from router.Node) error {
	pkt, ok := p.(*msg.Packet)
	if !ok {
		return msg.ErrInvalidPayload
	}
}

// SetHandler assigns the request Handler for this service.
func (s *service) SetHandler(h Handler) {
	s.HandlerLock.Lock()
	defer s.HandlerLock.Unlock()
	s.Handler = h
}

// GetHandler returns the request Handler for this service.
func (s *service) GetHandler() Handler {
	s.HandlerLock.RLock()
	defer s.HandlerLock.RUnlock()
	return s.Handler
}
