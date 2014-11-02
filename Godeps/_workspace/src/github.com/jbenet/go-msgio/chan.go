package msgio

import (
	"io"
	"sync"
)

type Chan struct {
	Buffers   [][]byte
	MsgChan   chan []byte
	ErrChan   chan error
	CloseChan chan bool
	BufPool   *sync.Pool
}

func NewChan(chanSize int) *Chan {
	return &Chan{
		MsgChan:   make(chan []byte, chanSize),
		ErrChan:   make(chan error, 1),
		CloseChan: make(chan bool, 2),
	}
}

func NewChanWithPool(chanSize int, pool *sync.Pool) *Chan {
	return &Chan{
		MsgChan:   make(chan []byte, chanSize),
		ErrChan:   make(chan error, 1),
		CloseChan: make(chan bool, 2),
		BufPool:   pool,
	}
}

func (s *Chan) getBuffer(size int) []byte {
	if s.BufPool == nil {
		return make([]byte, size)
	} else {
		bufi := s.BufPool.Get()
		buf, ok := bufi.([]byte)
		if !ok {
			panic("Got invalid type from sync pool!")
		}
		return buf
	}
}

func (s *Chan) ReadFrom(r io.Reader, maxMsgLen int) {
	// new buffer per message
	// if bottleneck, cycle around a set of buffers
	mr := NewReader(r)
Loop:
	for {
		buf := s.getBuffer(maxMsgLen)
		l, err := mr.ReadMsg(buf)
		if err != nil {
			if err == io.EOF {
				break Loop // done
			}

			// unexpected error. tell the client.
			s.ErrChan <- err
			break Loop
		}

		select {
		case <-s.CloseChan:
			break Loop // told we're done
		case s.MsgChan <- buf[:l]:
			// ok seems fine. send it away
		}
	}

	close(s.MsgChan)
	// signal we're done
	s.CloseChan <- true
}

func (s *Chan) WriteTo(w io.Writer) {
	// new buffer per message
	// if bottleneck, cycle around a set of buffers
	mw := NewWriter(w)
Loop:
	for {
		select {
		case <-s.CloseChan:
			break Loop // told we're done

		case msg, ok := <-s.MsgChan:
			if !ok { // chan closed
				break Loop
			}

			if err := mw.WriteMsg(msg); err != nil {
				if err != io.EOF {
					// unexpected error. tell the client.
					s.ErrChan <- err
				}

				break Loop
			}
		}
	}

	// signal we're done
	s.CloseChan <- true
}

func (s *Chan) Close() {
	s.CloseChan <- true
}
