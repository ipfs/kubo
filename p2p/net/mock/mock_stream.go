package mocknet

import (
	"io"

	inet "github.com/jbenet/go-ipfs/p2p/net"
)

// stream implements inet.Stream
type stream struct {
	io.Reader
	io.Writer
	conn *conn
}

func (s *stream) Close() error {
	s.conn.removeStream(s)
	if r, ok := (s.Reader).(io.Closer); ok {
		r.Close()
	}
	if w, ok := (s.Writer).(io.Closer); ok {
		return w.Close()
	}
	return nil
}

func (s *stream) Conn() inet.Conn {
	return s.conn
}
