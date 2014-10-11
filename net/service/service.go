package service

import (
	"errors"
	"sync"

	msg "github.com/jbenet/go-ipfs/net/message"
	u "github.com/jbenet/go-ipfs/util"

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

	// Start + Stop Service
	Start(ctx context.Context) error
	Stop()

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
	Handler Handler

	// Requests are all the pending requests on this service.
	Requests     RequestMap
	RequestsLock sync.RWMutex

	// cancel is the function to stop the Service
	cancel context.CancelFunc

	// Message Pipe (connected to the outside world)
	*msg.Pipe
}

// NewService creates a service object with given type ID and Handler
func NewService(h Handler) Service {
	return &service{
		Handler:  h,
		Requests: RequestMap{},
		Pipe:     msg.NewPipe(10),
	}
}

// Start kicks off the Service goroutines.
func (s *service) Start(ctx context.Context) error {
	if s.cancel != nil {
		return errors.New("Service already started.")
	}

	// make a cancellable context.
	ctx, s.cancel = context.WithCancel(ctx)

	go s.handleIncomingMessages(ctx)
	return nil
}

// Stop stops Service activity.
func (s *service) Stop() {
	s.cancel()
	s.cancel = context.CancelFunc(nil)
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

	// create a request
	r, err := NewRequest(m.Peer().ID)
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
func (s *service) handleIncomingMessages(ctx context.Context) {
	for {
		select {
		case m, more := <-s.Incoming:
			if !more {
				return
			}
			go s.handleIncomingMessage(ctx, m)

		case <-ctx.Done():
			return
		}
	}
}

func (s *service) handleIncomingMessage(ctx context.Context, m msg.NetMessage) {

	// unwrap the incoming message
	data, rid, err := unwrapData(m.Data())
	if err != nil {
		log.Error("de-serializing error: %v", err)
	}
	m2 := msg.New(m.Peer(), data)

	// if it's a request (or has no RequestID), handle it
	if rid == nil || rid.IsRequest() {
		if s.Handler == nil {
			log.Error("service dropped msg: %v", m)
			return // no handler, drop it.
		}

		// should this be "go HandleMessage ... ?"
		r1 := s.Handler.HandleMessage(ctx, m2)

		// if handler gave us a response, send it back out!
		if r1 != nil {
			err := s.sendMessage(ctx, r1, rid.Response())
			if err != nil {
				log.Error("error sending response message: %v", err)
			}
		}
		return
	}

	// Otherwise, it is a response. handle it.
	if !rid.IsResponse() {
		log.Error("RequestID should identify a response here.")
	}

	key := RequestKey(m.Peer().ID, RequestID(rid))
	s.RequestsLock.RLock()
	r, found := s.Requests[key]
	s.RequestsLock.RUnlock()

	if !found {
		log.Error("no request key %v (timeout?)", []byte(key))
		return
	}

	select {
	case r.Response <- m2:
	case <-ctx.Done():
	}
}

// SetHandler assigns the request Handler for this service.
func (s *service) SetHandler(h Handler) {
	s.Handler = h
}

// GetHandler returns the request Handler for this service.
func (s *service) GetHandler() Handler {
	return s.Handler
}
