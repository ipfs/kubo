package msgio

import (
	"io"
)

type Chan struct {
	Buffers   [][]byte
	MsgChan   chan []byte
	ErrChan   chan error
	CloseChan chan bool
}

func NewChan(chanSize int) *Chan {
	return &Chan{
		MsgChan:   make(chan []byte, chanSize),
		ErrChan:   make(chan error, 1),
		CloseChan: make(chan bool, 2),
	}
}

func (s *Chan) ReadFrom(r io.Reader, maxMsgLen int) {
	// new buffer per message
	// if bottleneck, cycle around a set of buffers
	mr := NewReader(r)
Loop:
	for {
		buf := make([]byte, maxMsgLen)
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
