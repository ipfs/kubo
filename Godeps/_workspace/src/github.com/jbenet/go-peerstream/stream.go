package peerstream

import (
	ss "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/spdystream"
)

// StreamHandler is a function which receives a Stream. It
// allows clients to set a function to receive newly created
// streams, and decide whether to continue adding them.
// It works sort of like a http.HandleFunc.
// Note: the StreamHandler is called sequentially, so spawn
// goroutines or pass the Stream. See EchoHandler.
type StreamHandler func(s *Stream)

// Stream is an io.{Read,Write,Close}r to a remote counterpart.
// It wraps a spdystream.Stream, and links it to a Conn and groups
type Stream struct {
	ssStream *ss.Stream

	conn   *Conn
	groups groupSet
}

func newStream(ssS *ss.Stream, c *Conn) *Stream {
	s := &Stream{
		conn:     c,
		ssStream: ssS,
		groups:   groupSet{m: make(map[Group]struct{})},
	}
	s.groups.AddSet(&c.groups) // inherit groups
	return s
}

// SPDYStream returns the underlying *spdystream.Stream
func (s *Stream) SPDYStream() *ss.Stream {
	return s.ssStream
}

// Conn returns the Conn associated with this Stream
func (s *Stream) Conn() *Conn {
	return s.conn
}

// Swarm returns the Swarm asociated with this Stream
func (s *Stream) Swarm() *Swarm {
	return s.conn.swarm
}

// Groups returns the Groups this Stream belongs to
func (s *Stream) Groups() []Group {
	return s.groups.Groups()
}

// InGroup returns whether this stream belongs to a Group
func (s *Stream) InGroup(g Group) bool {
	return s.groups.Has(g)
}

// AddGroup assigns given Group to Stream
func (s *Stream) AddGroup(g Group) {
	s.groups.Add(g)
}

// Write writes bytes to a stream, calling write data for each call.
func (s *Stream) Wait() error {
	return s.ssStream.Wait()
}

func (s *Stream) Read(p []byte) (n int, err error) {
	return s.ssStream.Read(p)
}

func (s *Stream) Write(p []byte) (n int, err error) {
	return s.ssStream.Write(p)
}

func (s *Stream) Close() error {
	return s.conn.swarm.removeStream(s)
}

// StreamsWithGroup narrows down a set of streams to those in given group.
func StreamsWithGroup(g Group, streams []*Stream) []*Stream {
	var out []*Stream
	for _, s := range streams {
		if s.InGroup(g) {
			out = append(out, s)
		}
	}
	return out
}
