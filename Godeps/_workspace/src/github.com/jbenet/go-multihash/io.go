package multihash

import (
	"fmt"
	"io"
)

// Reader is an io.Reader wrapper that exposes a function
// to read a whole multihash, parse it, and return it.
type Reader interface {
	io.Reader

	ReadMultihash() (Multihash, error)
}

// Writer is an io.Writer wrapper that exposes a function
// to write a whole multihash.
type Writer interface {
	io.Writer

	WriteMultihash(Multihash) error
}

// NewReader wraps an io.Reader with a multihash.Reader
func NewReader(r io.Reader) Reader {
	return &mhReader{r}
}

// NewWriter wraps an io.Writer with a multihash.Writer
func NewWriter(w io.Writer) Writer {
	return &mhWriter{w}
}

type mhReader struct {
	r io.Reader
}

func (r *mhReader) Read(buf []byte) (n int, err error) {
	return r.r.Read(buf)
}

func (r *mhReader) ReadMultihash() (Multihash, error) {
	mhhdr := make([]byte, 2)
	if _, err := io.ReadFull(r.r, mhhdr); err != nil {
		return nil, err
	}

	// first byte is the algo, the second is the length.

	// (varints someday...)
	length := uint(mhhdr[1])

	if length > 127 {
		return nil, fmt.Errorf("varints not yet supported (length is %d)", length)
	}

	buf := make([]byte, length+2)
	buf[0] = mhhdr[0]
	buf[1] = mhhdr[1]

	if _, err := io.ReadFull(r.r, buf[2:]); err != nil {
		return nil, err
	}

	return Cast(buf)
}

type mhWriter struct {
	w io.Writer
}

func (w *mhWriter) Write(buf []byte) (n int, err error) {
	return w.w.Write(buf)
}

func (w *mhWriter) WriteMultihash(m Multihash) error {
	_, err := w.w.Write([]byte(m))
	return err
}
