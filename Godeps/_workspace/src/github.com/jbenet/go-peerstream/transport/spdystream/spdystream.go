package peerstream_spdystream

import (
	"net"
	"net/http"

	pst "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport"
	ss "github.com/jbenet/spdystream"
)

// stream implements pst.Stream using a ss.Stream
type stream ss.Stream

func (s *stream) spdyStream() *ss.Stream {
	return (*ss.Stream)(s)
}

func (s *stream) Read(buf []byte) (int, error) {
	return s.spdyStream().Read(buf)
}

func (s *stream) Write(buf []byte) (int, error) {
	return s.spdyStream().Write(buf)
}

func (s *stream) Close() error {
	// Reset is spdystream's full bidirectional close.
	// We expose bidirectional close as our `Close`.
	// To close only half of the connection, and use other
	// spdystream options, just get the stream with:
	//  ssStream := (*ss.Stream)(stream)
	return s.spdyStream().Reset()
}

// Conn is a connection to a remote peer.
type conn ss.Connection

func (c *conn) spdyConn() *ss.Connection {
	return (*ss.Connection)(c)
}

func (c *conn) Close() error {
	return c.spdyConn().Close()
}

// OpenStream creates a new stream.
func (c *conn) OpenStream() (pst.Stream, error) {
	s, err := c.spdyConn().CreateStream(http.Header{}, nil, false)
	if err != nil {
		return nil, err
	}

	// wait for a response before writing. for some reason
	// spdystream does not make forward progress unless you do this.
	s.Wait()
	return (*stream)(s), nil
}

// Serve starts listening for incoming requests and handles them
// using given StreamHandler
func (c *conn) Serve(handler pst.StreamHandler) {
	c.spdyConn().Serve(func(s *ss.Stream) {

		// Flow control and backpressure of Opening streams is broken.
		// I believe that spdystream has one set of workers that both send
		// data AND accept new streams (as it's just more data). there
		// is a problem where if the new stream handlers want to throttle,
		// they also eliminate the ability to read/write data, which makes
		// forward-progress impossible. Thus, throttling this function is
		// -- at this moment -- not the solution. Either spdystream must
		// change, or we must throttle another way. go-peerstream handles
		// every new stream in its own goroutine.
		go func() {
			s.SendReply(http.Header{}, false)
			handler((*stream)(s))
		}()
	})
}

type transport struct{}

// Transport is a go-peerstream transport that constructs
// spdystream-backed connections.
var Transport = transport{}

func (t transport) NewConn(nc net.Conn, isServer bool) (pst.Conn, error) {
	c, err := ss.NewConnection(nc, isServer)
	return (*conn)(c), err
}
