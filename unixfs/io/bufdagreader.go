package io

import (
	"bytes"
	"context"
)

// BufDagReader implements a DagReader that reads from a byte slice
// using a bytes.Reader. It is used for RawNodes.
type BufDagReader struct {
	*bytes.Reader
}

// NewBufDagReader returns a DAG reader for the given byte slice.
// BufDagReader is used to read RawNodes.
func NewBufDagReader(b []byte) *BufDagReader {
	return &BufDagReader{bytes.NewReader(b)}
}

var _ DagReader = (*BufDagReader)(nil)

// Close is a nop.
func (*BufDagReader) Close() error {
	return nil
}

// CtxReadFull reads the slice onto b.
func (rd *BufDagReader) CtxReadFull(ctx context.Context, b []byte) (int, error) {
	return rd.Read(b)
}

// Size returns the size of the buffer.
func (rd *BufDagReader) Size() uint64 {
	s := rd.Reader.Size()
	if s < 0 {
		panic("size smaller than 0 (impossible!!)")
	}
	return uint64(s)
}
