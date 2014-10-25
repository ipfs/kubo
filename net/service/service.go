package service

import (
	"errors"
	"fmt"
	"sync"

	msg "github.com/jbenet/go-ipfs/net/message"
	u "github.com/jbenet/go-ipfs/util"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
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
	Sender
	ctxc.ContextCloser

	// GetPipe
	GetPipe() *msg.Pipe

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

	// Message Pipe (connected to the outside world)
	*msg.Pipe
	ctxc.ContextCloser
}

// NewService creates a service object with given type ID and Handler
func NewService(ctx context.Context, h Handler) Service {
	s := &service{
		Handler:       h,
		Requests:      RequestMap{},
		Pipe:          msg.NewPipe(10),
		ContextCloser: ctxc.NewContextCloser(ctx, nil),
	}

	s.Children().Add(1)
	go s.handleIncomingMessages()
	return s
}

// GetPipe implements the mux.Protocol interface
func (s *service) GetPipe() *msg.Pipe {
	return s.Pipe
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
	select {
	case s.Outgoing <- m2:
	case <-ctx.Done():
		return ctx.Err()
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

func (s *service) handleIncomingMessage(m msg.NetMessage) {
	defer s.Children().Done()

	// unwrap the incoming message
	data, rid, err := unwrapData(m.Data())
	if err != nil {
		log.Errorf("service de-serializing error: %v", err)
		return
	}

	m2 := msg.New(m.Peer(), data)

	// if it's a request (or has no RequestID), handle it
	if rid == nil || rid.IsRequest() {
		handler := s.GetHandler()
		if handler == nil {
			log.Errorf("service dropped msg: %v", m)
			return // no handler, drop it.
		}

		// should this be "go HandleMessage ... ?"
		r1 := handler.HandleMessage(s.Context(), m2)

		// if handler gave us a response, send it back out!
		if r1 != nil {
			err := s.sendMessage(s.Context(), r1, rid.Response())
			if err != nil {
				log.Errorf("error sending response message: %v", err)
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
