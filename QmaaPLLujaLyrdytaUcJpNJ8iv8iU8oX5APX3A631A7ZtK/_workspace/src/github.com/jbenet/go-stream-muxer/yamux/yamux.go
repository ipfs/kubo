package sm_yamux

import (
	"io/ioutil"
	"net"
	"time"

	yamux "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/hashicorp/yamux"
	smux "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer"
)

// stream implements smux.Stream using a ss.Stream
type stream yamux.Stream

func (s *stream) yamuxStream() *yamux.Stream {
	return (*yamux.Stream)(s)
}

func (s *stream) Read(buf []byte) (int, error) {
	return s.yamuxStream().Read(buf)
}

func (s *stream) Write(buf []byte) (int, error) {
	return s.yamuxStream().Write(buf)
}

func (s *stream) Close() error {
	return s.yamuxStream().Close()
}

// Conn is a connection to a remote peer.
type conn yamux.Session

func (c *conn) yamuxSession() *yamux.Session {
	return (*yamux.Session)(c)
}

func (c *conn) Close() error {
	return c.yamuxSession().Close()
}

func (c *conn) IsClosed() bool {
	return c.yamuxSession().IsClosed()
}

// OpenStream creates a new stream.
func (c *conn) OpenStream() (smux.Stream, error) {
	s, err := c.yamuxSession().OpenStream()
	if err != nil {
		return nil, err
	}

	return (*stream)(s), nil
}

// AcceptStream accepts a stream opened by the other side.
func (c *conn) AcceptStream() (smux.Stream, error) {
	s, err := c.yamuxSession().AcceptStream()
	return (*stream)(s), err
}

// Serve starts listening for incoming requests and handles them
// using given StreamHandler
func (c *conn) Serve(handler smux.StreamHandler) {
	for { // accept loop
		s, err := c.AcceptStream()
		if err != nil {
			return // err always means closed.
		}
		go handler(s)
	}
}

// Transport is a go-peerstream transport that constructs
// yamux-backed connections.
type Transport yamux.Config

// DefaultTransport has default settings for yamux
var DefaultTransport = (*Transport)(&yamux.Config{
	AcceptBacklog:       256,                // from yamux.DefaultConfig
	EnableKeepAlive:     true,               // from yamux.DefaultConfig
	KeepAliveInterval:   30 * time.Second,   // from yamux.DefaultConfig
	MaxStreamWindowSize: uint32(256 * 1024), // from yamux.DefaultConfig
	LogOutput:           ioutil.Discard,
})

func (t *Transport) NewConn(nc net.Conn, isServer bool) (smux.Conn, error) {
	var s *yamux.Session
	var err error
	if isServer {
		s, err = yamux.Server(nc, t.Config())
	} else {
		s, err = yamux.Client(nc, t.Config())
	}
	return (*conn)(s), err
}

func (t *Transport) Config() *yamux.Config {
	return (*yamux.Config)(t)
}
