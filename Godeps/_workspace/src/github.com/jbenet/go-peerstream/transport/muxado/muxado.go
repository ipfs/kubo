package peerstream_muxado

import (
	"net"

	muxado "github.com/inconshreveable/muxado"
	pst "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport"
)

// stream implements pst.Stream using a ss.Stream
type stream struct {
	ms muxado.Stream
}

func (s *stream) muxadoStream() muxado.Stream {
	return s.ms
}

func (s *stream) Read(buf []byte) (int, error) {
	return s.ms.Read(buf)
}

func (s *stream) Write(buf []byte) (int, error) {
	return s.ms.Write(buf)
}

func (s *stream) Close() error {
	return s.ms.Close()
}

// Conn is a connection to a remote peer.
type conn struct {
	ms muxado.Session
}

func (c *conn) muxadoSession() muxado.Session {
	return c.ms
}

func (c *conn) Close() error {
	return c.ms.Close()
}

// OpenStream creates a new stream.
func (c *conn) OpenStream() (pst.Stream, error) {
	s, err := c.ms.Open()
	if err != nil {
		return nil, err
	}

	return &stream{ms: s}, nil
}

// Serve starts listening for incoming requests and handles them
// using given StreamHandler
func (c *conn) Serve(handler pst.StreamHandler) {
	for { // accept loop
		s, err := c.ms.Accept()
		if err != nil {
			return // err always means closed.
		}
		go handler(&stream{ms: s})
	}
}

type transport struct{}

// Transport is a go-peerstream transport that constructs
// spdystream-backed connections.
var Transport = transport{}

func (t transport) NewConn(nc net.Conn, isServer bool) (pst.Conn, error) {
	var s muxado.Session
	if isServer {
		s = muxado.Server(nc)
	} else {
		s = muxado.Client(nc)
	}
	return &conn{ms: s}, nil
}
