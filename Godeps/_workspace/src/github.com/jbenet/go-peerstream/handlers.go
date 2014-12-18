package peerstream

import (
	"io"
	"math/rand"
)

var SelectRandomConn = func(conns []*Conn) *Conn {
	if len(conns) == 0 {
		return nil
	}

	return conns[rand.Intn(len(conns))]
}

func EchoHandler(s *Stream) {
	go func() {
		io.Copy(s, s)
		s.Close()
	}()
}

func CloseHandler(s *Stream) {
	s.Close()
}

func NoOpStreamHandler(s *Stream) {}

func NoOpConnHandler(c *Conn) {}
