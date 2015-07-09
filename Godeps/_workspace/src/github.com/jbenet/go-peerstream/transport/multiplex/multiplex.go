package peerstream_multiplex

import (
	"net"

	pst "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport"
	mp "github.com/whyrusleeping/go-multiplex"
)

type conn struct {
	*mp.Multiplex
}

func ( // Conn is a connection to a remote peer.
c *conn) Close() error {
	return c.Multiplex.Close()
}

func (c *conn) IsClosed() bool {
	return c.Multiplex.IsClosed()
}

// OpenStream creates a new stream.
func (c *conn) OpenStream() (pst.Stream, error) {
	return c.Multiplex.NewStream(), nil
}

// Serve starts listening for incoming requests and handles them
// using given StreamHandler
func (c *conn) Serve(handler pst.StreamHandler) {
	c.Multiplex.Serve(func(s *mp.Stream) {
		handler(s)
	})
}

// Transport is a go-peerstream transport that constructs
// multiplex-backed connections.
type Transport struct{}

// DefaultTransport has default settings for multiplex
var DefaultTransport = &Transport{}

func (t *Transport) NewConn(nc net.Conn, isServer bool) (pst.Conn, error) {
	return &conn{mp.NewMultiplex(nc, isServer)}, nil
}
