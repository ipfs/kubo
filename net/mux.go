package net

import (
	"errors"
	"fmt"
	"io"
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"
)

var log = eventlog.Logger("mux2")

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
	Default  StreamHandler // handles unknown protocols.
	Handlers StreamHandlerMap

	sync.RWMutex
}

// Protocols returns the list of protocols this muxer has handlers for
func (m *Mux) Protocols() []ProtocolID {
	m.RLock()
	l := make([]ProtocolID, 0, len(m.Handlers))
	for p := range m.Handlers {
		l = append(l, p)
	}
	m.RUnlock()
	return l
}

// ReadProtocolHeader reads the stream and returns the next Handler function
// according to the muxer encoding.
func (m *Mux) ReadProtocolHeader(s io.Reader) (string, StreamHandler, error) {
	// log.Error("ReadProtocolHeader")
	name, err := ReadLengthPrefix(s)
	if err != nil {
		return "", nil, err
	}

	// log.Debug("ReadProtocolHeader got:", name)
	m.RLock()
	h, found := m.Handlers[ProtocolID(name)]
	m.RUnlock()

	switch {
	case !found && m.Default != nil:
		return name, m.Default, nil
	case !found && m.Default == nil:
		return name, nil, errors.New("no handler with name: " + name)
	default:
		return name, h, nil
	}
}

// SetHandler sets the protocol handler on the Network's Muxer.
// This operation is threadsafe.
func (m *Mux) SetHandler(p ProtocolID, h StreamHandler) {
	m.Lock()
	m.Handlers[p] = h
	m.Unlock()
}

// Handle reads the next name off the Stream, and calls a function
func (m *Mux) Handle(s Stream) {

	// Flow control and backpressure of Opening streams is broken.
	// I believe that spdystream has one set of workers that both send
	// data AND accept new streams (as it's just more data). there
	// is a problem where if the new stream handlers want to throttle,
	// they also eliminate the ability to read/write data, which makes
	// forward-progress impossible. Thus, throttling this function is
	// -- at this moment -- not the solution. Either spdystream must
	// change, or we must throttle another way.
	//
	// In light of this, we use a goroutine for now (otherwise the
	// spdy worker totally blocks, and we can't even read the protocol
	// header). The better route in the future is to use a worker pool.
	go func() {
		ctx := context.Background()

		name, handler, err := m.ReadProtocolHeader(s)
		if err != nil {
			err = fmt.Errorf("protocol mux error: %s", err)
			log.Error(err)
			log.Event(ctx, "muxError", lgbl.Error(err))
			return
		}

		log.Info("muxer handle protocol: %s", name)
		log.Event(ctx, "muxHandle", eventlog.Metadata{"protocol": name})
		handler(s)
	}()
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
