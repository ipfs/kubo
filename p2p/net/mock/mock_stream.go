package mocknet

import (
	"bytes"
	"fmt"
	inet "github.com/ipfs/go-ipfs/p2p/net"
	"io"
	"time"
)

// stream implements inet.Stream
type stream struct {
	io.Reader
	io.Writer
	conn      *conn
	toDeliver chan *transportObject
	done      chan bool
}

type transportObject struct {
	msg         []byte
	arrivalTime time.Time
}

//  How to handle errors with writes?
func (s *stream) Write(p []byte) (n int, err error) {
	l := s.conn.link
	delay := l.GetLatency() + l.RateLimit(len(p))
	t := time.Now().Add(delay)
	s.toDeliver <- &transportObject{msg: p, arrivalTime: t}
	return len(p), nil
}

func (s *stream) Close() error {
	s.conn.removeStream(s)
	close(s.toDeliver)
	//  wait for transport to finish writing/sleeping before closing stream
	<-s.done
	if r, ok := (s.Reader).(io.Closer); ok {
		r.Close()
	}
	if w, ok := (s.Writer).(io.Closer); ok {
		w.Close()
	}
	s.conn.net.notifyAll(func(n inet.Notifiee) {
		n.ClosedStream(s.conn.net, s)
	})
	return nil
}

func (s *stream) Conn() inet.Conn {
	return s.conn
}

// transport will grab message arrival times, wait until that time, and
// then write the message out when it is scheduled to arrive
func (s *stream) transport() {
	bufsize := 256
	buf := new(bytes.Buffer)
	ticker := time.NewTicker(time.Millisecond * 4)
loop:
	for {
		select {
		case o, ok := <-s.toDeliver:
			if !ok {
				close(s.done)
				return
			}

			buffered := len(o.msg) + buf.Len()

			now := time.Now()
			if now.Before(o.arrivalTime) {
				if buffered < bufsize {
					buf.Write(o.msg)
					continue loop
				} else {
					time.Sleep(o.arrivalTime.Sub(now))
				}
			}

			if buf.Len() > 0 {
				_, err := s.Writer.Write(buf.Bytes())
				if err != nil {
					return
				}
				buf.Reset()
			}

			_, err := s.Writer.Write(o.msg)
			if err != nil {
				fmt.Println("mock_stream", err)
			}

		case <-ticker.C:
			if buf.Len() > 0 {
				_, err := s.Writer.Write(buf.Bytes())
				if err != nil {
					return
				}
				buf.Reset()
			}
		}
	}
}
