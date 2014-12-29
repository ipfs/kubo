package mux

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

// Mux provides simple stream multixplexing.
// It helps you precisely when:
//  * You have many streams
//  * You have function handlers
//
// It contains the handlers for each protocol accepted.
// It dispatches handlers for streams opened by remote peers.
//
// We use a totally ad-hoc encoding:
//   <1 byte length in bytes><string name>
// So "bitswap" is 0x0762697473776170
//
// NOTE: only the dialer specifies this muxing line.
// This is because we're using Streams :)
//
// WARNING: this datastructure IS NOT threadsafe.
// do not modify it once the network is using it.
type Mux struct {
	Default  inet.StreamHandler // handles unknown protocols.
	Handlers inet.StreamHandlerMap

	sync.RWMutex
}

// Protocols returns the list of protocols this muxer has handlers for
func (m *Mux) Protocols() []inet.ProtocolID {
	m.RLock()
	l := make([]inet.ProtocolID, 0, len(m.Handlers))
	for p := range m.Handlers {
		l = append(l, p)
	}
	m.RUnlock()
	return l
}

// ReadProtocolHeader reads the stream and returns the next Handler function
// according to the muxer encoding.
func (m *Mux) ReadProtocolHeader(s io.Reader) (string, inet.StreamHandler, error) {
	// log.Error("ReadProtocolHeader")
	name, err := ReadLengthPrefix(s)
	if err != nil {
		return "", nil, err
	}

	// log.Debug("ReadProtocolHeader got:", name)
	m.RLock()
	h, found := m.Handlers[inet.ProtocolID(name)]
	m.RUnlock()

	switch {
	case !found && m.Default != nil:
		return name, m.Default, nil
	case !found && m.Default == nil:
		return name, nil, fmt.Errorf("%s no handler with name: %s (%d)", m, name, len(name))
	default:
		return name, h, nil
	}
}

// String returns the muxer's printing representation
func (m *Mux) String() string {
	m.RLock()
	defer m.RUnlock()
	return fmt.Sprintf("<Muxer %p %d>", m, len(m.Handlers))
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (m *Mux) SetHandler(p inet.ProtocolID, h inet.StreamHandler) {
	log.Debugf("%s setting handler for protocol: %s (%d)", m, p, len(p))
	m.Lock()
	m.Handlers[p] = h
	m.Unlock()
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

	name, handler, err := m.ReadProtocolHeader(s)
	if err != nil {
		err = fmt.Errorf("protocol mux error: %s", err)
		log.Error(err)
		log.Event(ctx, "muxError", lgbl.Error(err))
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

// WriteLengthPrefix writes the name into Writer with a length-byte-prefix.
func WriteLengthPrefix(w io.Writer, name string) error {
	// log.Error("WriteLengthPrefix", name)
	s := make([]byte, len(name)+1)
	s[0] = byte(len(name))
	copy(s[1:], []byte(name))

	_, err := w.Write(s)
	return err
}

// WriteProtocolHeader defines how a protocol is written into the header of
// a stream. This is so the muxer can multiplex between services.
func WriteProtocolHeader(pr inet.ProtocolID, s inet.Stream) error {
	if pr != "" { // only write proper protocol headers
		if err := WriteLengthPrefix(s, string(pr)); err != nil {
			return err
		}
	}
	return nil
}
