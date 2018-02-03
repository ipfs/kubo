// Package chunk implements streaming block splitters.
// Splitters read data from a reader and provide byte slices (chunks)
// The size and contents of these slices depend on the splitting method
// used.
package chunk

import (
	"io"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	mpool "gx/ipfs/QmWBug6eBS7AxRdCDVuSY5CnSit7cS2XnPFYJWqWDumhCG/go-msgio/mpool"
)

var log = logging.Logger("chunk")

// DefaultBlockSize is the chunk size that splitters produce (or aim to).
var DefaultBlockSize int64 = 1024 * 256

// A Splitter reads bytes from a Reader and creates "chunks" (byte slices)
// that can be used to build DAG nodes.
type Splitter interface {
	Reader() io.Reader
	NextBytes() ([]byte, error)
}

// SplitterGen is a splitter generator, given a reader.
type SplitterGen func(r io.Reader) Splitter

// DefaultSplitter returns a SizeSplitter with the DefaultBlockSize.
func DefaultSplitter(r io.Reader) Splitter {
	return NewSizeSplitter(r, DefaultBlockSize)
}

// SizeSplitterGen returns a SplitterGen function which will create
// a splitter with the given size when called.
func SizeSplitterGen(size int64) SplitterGen {
	return func(r io.Reader) Splitter {
		return NewSizeSplitter(r, size)
	}
}

// Chan returns a channel that receives each of the chunks produced
// by a splitter, along with another one for errors.
func Chan(s Splitter) (<-chan []byte, <-chan error) {
	out := make(chan []byte)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)

		// all-chunks loop (keep creating chunks)
		for {
			b, err := s.NextBytes()
			if err != nil {
				errs <- err
				return
			}

			out <- b
		}
	}()
	return out, errs
}

type sizeSplitterv2 struct {
	r    io.Reader
	size uint32
	err  error
}

// NewSizeSplitter returns a new size-based Splitter with the given block size.
func NewSizeSplitter(r io.Reader, size int64) Splitter {
	return &sizeSplitterv2{
		r:    r,
		size: uint32(size),
	}
}

// NextBytes produces a new chunk.
func (ss *sizeSplitterv2) NextBytes() ([]byte, error) {
	if ss.err != nil {
		return nil, ss.err
	}

	full := mpool.ByteSlicePool.Get(ss.size).([]byte)[:ss.size]
	n, err := io.ReadFull(ss.r, full)
	switch err {
	case io.ErrUnexpectedEOF:
		ss.err = io.EOF
		small := make([]byte, n)
		copy(small, full)
		mpool.ByteSlicePool.Put(ss.size, full)
		return small, nil
	case nil:
		return full, nil
	default:
		mpool.ByteSlicePool.Put(ss.size, full)
		return nil, err
	}
}

// Reader returns the io.Reader associated to this Splitter.
func (ss *sizeSplitterv2) Reader() io.Reader {
	return ss.r
}
