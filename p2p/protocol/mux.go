package protocol

import (
	"fmt"
	"io"
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	inet "github.com/jbenet/go-ipfs/p2p/net"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"
)

var log = eventlog.Logger("net/mux")

type streamHandlerMap map[ID]inet.StreamHandler

// Mux provides simple stream multixplexing.
// It helps you precisely when:
//  * You have many streams
//  * You have function handlers
//
// It contains the handlers for each protocol accepted.
// It dispatches handlers for streams opened by remote peers.
type Mux struct {
	// defaultHandler handles unknown protocols. Callers modify at your own risk.
	defaultHandler inet.StreamHandler

	lock     sync.RWMutex
	handlers streamHandlerMap
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

// readHeader reads the stream and returns the next Handler function
// according to the muxer encoding.
func (m *Mux) readHeader(s io.Reader) (ID, inet.StreamHandler, error) {
	// log.Error("ReadProtocolHeader")
	p, err := ReadHeader(s)
	if err != nil {
		return "", nil, err
	}

	// log.Debug("readHeader got:", p)
	m.lock.RLock()
	h, found := m.handlers[p]
	m.lock.RUnlock()

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

	name, handler, err := m.readHeader(s)
	if err != nil {
		err = fmt.Errorf("protocol mux error: %s", err)
		log.Error(err)
		log.Event(ctx, "muxError", lgbl.Error(err))
		s.Close()
		return
	}

	log.Infof("muxer handle protocol: %s", name)
	log.Event(ctx, "muxHandle", eventlog.Metadata{"protocol": name})
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
