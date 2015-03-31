package swarm

import (
	inet "github.com/ipfs/go-ipfs/p2p/net"

	ps "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream"
)

// a Stream is a wrapper around a ps.Stream that exposes a way to get
// our Conn and Swarm (instead of just the ps.Conn and ps.Swarm)
type Stream ps.Stream

// Stream returns the underlying peerstream.Stream
func (s *Stream) Stream() *ps.Stream {
	return (*ps.Stream)(s)
}

// Conn returns the Conn associated with this Stream, as an inet.Conn
func (s *Stream) Conn() inet.Conn {
	return s.SwarmConn()
}

// SwarmConn returns the Conn associated with this Stream, as a *Conn
func (s *Stream) SwarmConn() *Conn {
	return (*Conn)(s.Stream().Conn())
}

// Read reads bytes from a stream.
func (s *Stream) Read(p []byte) (n int, err error) {
	return s.Stream().Read(p)
}

// Write writes bytes to a stream, flushing for each call.
func (s *Stream) Write(p []byte) (n int, err error) {
	return s.Stream().Write(p)
}

// Close closes the stream, indicating this side is finished
// with the stream.
func (s *Stream) Close() error {
	return s.Stream().Close()
}

func wrapStream(pss *ps.Stream) *Stream {
	return (*Stream)(pss)
}

func wrapStreams(st []*ps.Stream) []*Stream {
	out := make([]*Stream, len(st))
	for i, s := range st {
		out[i] = wrapStream(s)
	}
	return out
}
