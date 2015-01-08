package peerstream_transport

import (
	"io"
	"net"
)

// Stream is a bidirectional io pipe within a connection
type Stream interface {
	io.Reader
	io.Writer
	io.Closer
}

// StreamHandler is a function that handles streams
// (usually those opened by the remote side)
type StreamHandler func(Stream)

// Conn is a stream-multiplexing connection to a remote peer.
type Conn interface {
	io.Closer

	// IsClosed returns whether a connection is fully closed, so it can
	// be garbage collected.
	IsClosed() bool

	// OpenStream creates a new stream.
	OpenStream() (Stream, error)

	// Serve starts listening for incoming requests and handles them
	// using given StreamHandler
	Serve(StreamHandler)
}

// Transport constructs go-peerstream compatible connections.
type Transport interface {

	// NewConn constructs a new connection
	NewConn(c net.Conn, isServer bool) (Conn, error)
}
