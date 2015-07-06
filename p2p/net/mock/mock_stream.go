package mocknet

import (
	"bytes"
	"io"
	"time"

	process "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"

	inet "github.com/ipfs/go-ipfs/p2p/net"
)

// stream implements inet.Stream
type stream struct {
	io.Reader
	io.Writer
	conn      *conn
	toDeliver chan *transportObject
	proc      process.Process
}

type transportObject struct {
	msg         []byte
	arrivalTime time.Time
}

func NewStream(w io.Writer, r io.Reader) *stream {
	s := &stream{
		Reader:    r,
		Writer:    w,
		toDeliver: make(chan *transportObject),
	}

	s.proc = process.WithTeardown(s.teardown)
	s.proc.Go(s.transport)
	return s
}

//  How to handle errors with writes?
func (s *stream) Write(p []byte) (n int, err error) {
	l := s.conn.link
	delay := l.GetLatency() + l.RateLimit(len(p))
	t := time.Now().Add(delay)
	select {
	case <-s.proc.Closing(): // bail out if we're closing.
		return 0, io.ErrClosedPipe
	case s.toDeliver <- &transportObject{msg: p, arrivalTime: t}:
	}
	return len(p), nil
}

func (s *stream) Close() error {
	return s.proc.Close()
}

// teardown shuts down the stream. it is called by s.proc.Close()
// after all the children of this s.proc (i.e. transport's proc)
// are done.
func (s *stream) teardown() error {
	// at this point, no streams are writing.

	s.conn.removeStream(s)
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
func (s *stream) transport(proc process.Process) {
	bufsize := 256
	buf := new(bytes.Buffer)
	ticker := time.NewTicker(time.Millisecond * 4)

	// writeBuf writes the contents of buf through to the s.Writer.
	// done only when arrival time makes sense.
	drainBuf := func() {
		if buf.Len() > 0 {
			_, err := s.Writer.Write(buf.Bytes())
			if err != nil {
				return
			}
			buf.Reset()
		}
	}

	// deliverOrWait is a helper func that processes
	// an incoming packet. it waits until the arrival time,
	// and then writes things out.
	deliverOrWait := func(o *transportObject) {
		buffered := len(o.msg) + buf.Len()

		now := time.Now()
		if now.Before(o.arrivalTime) {
			if buffered < bufsize {
				buf.Write(o.msg)
				return
			}

			// we do not buffer + return here, instead hanging the
			// call (i.e. not accepting any more transportObjects)
			// so that we apply back-pressure to the sender.
			// this sleep should wake up same time as ticker.
			time.Sleep(o.arrivalTime.Sub(now))
		}

		// ok, we waited our due time. now rite the buf + msg.

		// drainBuf first, before we write this message.
		drainBuf()

		// write this message.
		_, err := s.Writer.Write(o.msg)
		if err != nil {
			log.Error("mock_stream", err)
		}
	}

	for {
		select {
		case <-proc.Closing():
			return // bail out of here.

		case o, ok := <-s.toDeliver:
			if !ok {
				return
			}
			deliverOrWait(o)

		case <-ticker.C: // ok, due to write it out.
			drainBuf()
		}
	}
}
