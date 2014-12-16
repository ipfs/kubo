package net

import (
	"errors"
	"fmt"
	"io"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"
)

var log = eventlog.Logger("mux2")

// NextName reads the stream and returns the next protocol name
// according to the muxer encoding.
func (m *Mux) NextName(s io.Reader) (string, error) {

	// c-string identifier
	// the first byte is our length
	l := make([]byte, 1)
	if _, err := io.ReadFull(s, l); err != nil {
		return "", err
	}
	length := int(l[0])

	// the next are our identifier
	name := make([]byte, length)
	if _, err := io.ReadFull(s, name); err != nil {
		return "", err
	}

	return string(name), nil
}

// NextHandler reads the stream and returns the next Handler function
// according to the muxer encoding.
func (m *Mux) NextHandler(s io.Reader) (string, StreamHandler, error) {
	name, err := m.NextName(s)
	if err != nil {
		return "", nil, err
	}

	h, found := m.Handlers[name]
	if !found {
		if m.Default == nil {
			return name, nil, errors.New("no handler with name: " + name)
		}

		return name, m.Default, nil
	}

	return name, h, nil
}

// Handle reads the next name off the Stream, and calls a function
func (m *Mux) Handle(s Stream) {
	ctx := context.Background()

	name, handler, err := m.NextHandler(s)
	if err != nil {
		err = fmt.Errorf("protocol mux error: %s", err)
		log.Error(err)
		log.Event(ctx, "muxError", lgbl.Error(err))
		return
	}

	log.Info("muxer handle protocol: %s", name)
	log.Event(ctx, "muxHandle", eventlog.Metadata{"protocol": name})
	handler(s)
}

// Write writes the name into Writer with a length-byte-prefix.
func Write(w io.Writer, name string) error {
	s := make([]byte, len(name)+1)
	s[0] = byte(len(name))
	copy(s[1:], []byte(name))

	_, err := w.Write(s)
	return err
}
