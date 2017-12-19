// package chunk implements streaming block splitters
package chunk

import (
	"io"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	mpool "gx/ipfs/QmWBug6eBS7AxRdCDVuSY5CnSit7cS2XnPFYJWqWDumhCG/go-msgio/mpool"
)

var log = logging.Logger("chunk")

var DefaultBlockSize int64 = 1024 * 256

type Splitter interface {
	Reader() io.Reader
	NextBytes() ([]byte, error)
}

type SplitterGen func(r io.Reader) Splitter

func DefaultSplitter(r io.Reader) Splitter {
	return NewSizeSplitter(r, DefaultBlockSize)
}

func SizeSplitterGen(size int64) SplitterGen {
	return func(r io.Reader) Splitter {
		return NewSizeSplitter(r, size)
	}
}

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

func NewSizeSplitter(r io.Reader, size int64) Splitter {
	return &sizeSplitterv2{
		r:    r,
		size: uint32(size),
	}
}

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

func (ss *sizeSplitterv2) Reader() io.Reader {
	return ss.r
}
