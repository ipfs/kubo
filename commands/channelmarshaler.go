package commands

import "io"

type ChannelMarshaler struct {
	Channel   <-chan interface{}
	Marshaler func(interface{}) (io.Reader, error)

	reader io.Reader
}

func (cr *ChannelMarshaler) Read(p []byte) (int, error) {
	if cr.reader == nil {
		val, more := <-cr.Channel
		if !more {
			return 0, io.EOF
		}

		r, err := cr.Marshaler(val)
		if err != nil {
			return 0, err
		}
		cr.reader = r
	}

	n, err := cr.reader.Read(p)
	if err != nil && err != io.EOF {
		return n, err
	}
	if n == 0 {
		cr.reader = nil
	}
	return n, nil
}
