package protocol

import (
	"fmt"
	"io"
	"sync"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	inet "github.com/ipfs/go-ipfs/p2p/net"
	logging "github.com/ipfs/go-ipfs/vendor/go-log-v1.0.0"
	lgbl "github.com/ipfs/go-ipfs/util/eventlog/loggables"
)

var log = logging.Logger("net/mux")

type streamHandlerMap map[ID]inet.StreamHandler

// Mux provides simple stream multixplexing.
// It helps you precisely when:
//  * You have many streams
//  * You have function handlers
//
// It contains the handlers for each protocol accepted.
// It dispatches handlers for streams opened by remote peers.
type Mux struct {
	lock           sync.RWMutex
	handlers       streamHandlerMap
	defaultHandler inet.StreamHandler
}

func NewMux() *Mux {
	return &Mux{
		handlers: streamHandlerMap{},
	}
}

// Protocols returns the list of protocols this muxer has handlers for
func (m *Mux) Protocols() []ID {
	m.lock.RLock()
	l := make([]ID, 0, len(m.handlers))
	for p := range m.handlers {
		l = append(l, p)
	}
	m.lock.RUnlock()
	return l
}

// ReadHeader reads the stream and returns the next Handler function
// according to the muxer encoding.
func (m *Mux) ReadHeader(s io.Reader) (ID, inet.StreamHandler, error) {
	p, err := ReadHeader(s)
	if err != nil {
		return "", nil, err
	}

	m.lock.RLock()
	defer m.lock.RUnlock()
	h, found := m.handlers[p]

	switch {
	case !found && m.defaultHandler != nil:
		return p, m.defaultHandler, nil
	case !found && m.defaultHandler == nil:
		return p, nil, fmt.Errorf("%s no handler with name: %s (%d)", m, p, len(p))
	default:
		return p, h, nil
	}
}

// String returns the muxer's printing representation
func (m *Mux) String() string {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return fmt.Sprintf("<Muxer %p %d>", m, len(m.handlers))
}

func (m *Mux) SetDefaultHandler(h inet.StreamHandler) {
	m.lock.Lock()
	m.defaultHandler = h
	m.lock.Unlock()
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (m *Mux) SetHandler(p ID, h inet.StreamHandler) {
	log.Debugf("%s setting handler for protocol: %s (%d)", m, p, len(p))
	m.lock.Lock()
	m.handlers[p] = h
	m.lock.Unlock()
}

// RemoveHandler removes the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (m *Mux) RemoveHandler(p ID) {
	log.Debugf("%s removing handler for protocol: %s (%d)", m, p, len(p))
	m.lock.Lock()
	delete(m.handlers, p)
	m.lock.Unlock()
}

// Handle reads the next name off the Stream, and calls a handler function
// This is done in its own goroutine, to avoid blocking the caller.
func (m *Mux) Handle(s inet.Stream) {
	go m.HandleSync(s)
}

// HandleSync reads the next name off the Stream, and calls a handler function
// This is done synchronously. The handler function will return before
// HandleSync returns.
func (m *Mux) HandleSync(s inet.Stream) {
	ctx := context.Background()

	name, handler, err := m.ReadHeader(s)
	if err != nil {
		err = fmt.Errorf("protocol mux error: %s", err)
		log.Event(ctx, "muxError", lgbl.Error(err))
		s.Close()
		return
	}

	log.Debugf("muxer handle protocol %s: %s", s.Conn().RemotePeer(), name)
	handler(s)
}

// ReadLengthPrefix reads the name from Reader with a length-byte-prefix.
func ReadLengthPrefix(r io.Reader) (string, error) {
	// c-string identifier
	// the first byte is our length
	l := make([]byte, 1)
	if _, err := io.ReadFull(r, l); err != nil {
		return "", err
	}
	length := int(l[0])

	// the next are our identifier
	name := make([]byte, length)
	if _, err := io.ReadFull(r, name); err != nil {
		return "", err
	}

	return string(name), nil
}
