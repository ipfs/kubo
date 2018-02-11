package io

import (
	"errors"
	"io"
)

func newSizeAdjReadSeekCloser(base ReadSeekCloser, size uint64) *sizeAdjReadSeekCloser {
	r := &sizeAdjReadSeekCloser{base: base, size: int64(size)}
	if r.size < 0 {
		panic("size limited 2^63 − 1")
	}
	return r
}

type sizeAdjReadSeekCloser struct {
	base   ReadSeekCloser
	size   int64
	offset int64
}

// Read implements the Read method as defined by io.Reader
func (r *sizeAdjReadSeekCloser) Read(p []byte) (int, error) {
	if r.offset >= r.size { // EOF
		return 0, io.EOF
	}
	if int64(len(p)) > r.size-r.offset { // truncate
		newsize := r.size - r.offset
		p = p[:newsize]
	}
	n, err := r.base.Read(p)
	if err == nil {
		_, err = r.base.Read(nil)
	}
	if err != io.EOF { // only pad when we get an EOF
		r.offset += int64(n)
		return n, err
	}
	for ; n < len(p) && r.offset+int64(n) < r.size; n++ { // pad
		p[n] = 0
	}
	r.offset += int64(n)
	return n, io.EOF
}

// Seek implements the Seek method as defined by io.Seeker
func (r *sizeAdjReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.offset = offset
	case io.SeekCurrent:
		r.offset += offset
	case io.SeekEnd:
		r.offset = r.size + offset
	}
	if r.offset < 0 {
		return -1, errors.New("Seek will result in negative position")
	}
	// Its easier just to always use io.SeekStart rather than
	// correctly adjust offset for io.SeekCurrent and io.SeekEnd.
	return r.base.Seek(r.offset, io.SeekStart)
}

// Close implements the Close method as defined by io.Closer
func (r *sizeAdjReadSeekCloser) Close() error {
	return r.base.Close()
}

// WriteTo implemented WriteTo method as defined by io.WriterTo
func (r *sizeAdjReadSeekCloser) WriteTo(w io.Writer) (int64, error) {
	lr := &truncWriter{base: w, size: r.size - r.offset}
	_, err := r.base.WriteTo(lr)
	n := lr.offset
	if err != nil {
		r.offset += n
		return n, err
	}
	if r.offset+n < r.size {
		zeros := make([]byte, r.size-(r.offset+n))
		n0, err0 := w.Write(zeros)
		n += int64(n0)
		err = err0
	}
	r.offset += n
	return n, err
}

// truncWriter accepts all bytes written to it, but discards the tail
// after size bytes are accepted
type truncWriter struct {
	base   io.Writer
	size   int64
	offset int64
}

// Write implemented Write method as defined by io.Writer
func (w *truncWriter) Write(p []byte) (int, error) {
	truncC := 0
	if int64(len(p)) > w.size-w.offset {
		truncC = int(int64(len(p)) - w.size - w.offset)
		p = p[:w.size]
	}
	if len(p) == 0 {
		return truncC, nil
	}
	n, err := w.base.Write(p)
	w.offset += int64(n)
	return n + truncC, err
}
