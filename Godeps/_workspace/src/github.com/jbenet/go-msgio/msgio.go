package msgio

import (
	"encoding/binary"
	"io"
)

var NBO = binary.BigEndian

type Writer interface {
	WriteMsg([]byte) error
}

type WriteCloser interface {
	Writer
	io.Closer
}

type Reader interface {
	ReadMsg([]byte) (int, error)
}

type ReadCloser interface {
	Reader
	io.Closer
}

type ReadWriter interface {
	Reader
	Writer
}

type ReadWriteCloser interface {
	Reader
	Writer
	io.Closer
}

type Writer_ struct {
	W io.Writer
}

func NewWriter(w io.Writer) WriteCloser {
	return &Writer_{w}
}

func (s *Writer_) WriteMsg(msg []byte) (err error) {
	length := uint32(len(msg))
	if err := binary.Write(s.W, NBO, &length); err != nil {
		return err
	}
	_, err = s.W.Write(msg)
	return err
}

func (s *Writer_) Close() error {
	if c, ok := s.W.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

type Reader_ struct {
	R    io.Reader
	lbuf []byte
}

func NewReader(r io.Reader) ReadCloser {
	return &Reader_{r, make([]byte, 4)}
}

func (s *Reader_) ReadMsg(msg []byte) (int, error) {
	if _, err := io.ReadFull(s.R, s.lbuf); err != nil {
		return 0, err
	}
	length := int(NBO.Uint32(s.lbuf))
	if length < 0 || length > len(msg) {
		return 0, io.ErrShortBuffer
	}
	_, err := io.ReadFull(s.R, msg[:length])
	return length, err
}

func (s *Reader_) Close() error {
	if c, ok := s.R.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

type ReadWriter_ struct {
	Reader
	Writer
}

func NewReadWriter(rw io.ReadWriter) ReadWriter {
	return &ReadWriter_{
		Reader: NewReader(rw),
		Writer: NewWriter(rw),
	}
}

func (rw *ReadWriter_) Close() error {
	if w, ok := rw.Writer.(WriteCloser); ok {
		return w.Close()
	}
	if r, ok := rw.Reader.(ReadCloser); ok {
		return r.Close()
	}
	return nil
}
