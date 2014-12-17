package swarm

import (
	ps "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream"
)

// a Stream is a wrapper around a ps.Stream that exposes a way to get
// our Conn and Swarm (instead of just the ps.Conn and ps.Swarm)
type Stream ps.Stream

// StreamHandler is called when new streams are opened from remote peers.
// See peerstream.StreamHandler
type StreamHandler func(*Stream)

// Stream returns the underlying peerstream.Stream
func (s *Stream) Stream() *ps.Stream {
	return (*ps.Stream)(s)
}

// Conn returns the Conn associated with this Stream
func (s *Stream) Conn() *Conn {
	return (*Conn)(s.Stream().Conn())
}

// Wait waits for the stream to receive a reply.
func (s *Stream) Wait() error {
	return s.Stream().Wait()
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
