// package chunk implements streaming block splitters
package chunk

import (
	"io"

	"github.com/ipfs/go-ipfs/commands/files"
	logging "gx/ipfs/QmaDNZ4QMdBdku1YZWBysufYyoQt1negQGNav6PLYarbY8/go-log"
)

var log = logging.Logger("chunk")

var DefaultBlockSize int64 = 1024 * 256

type Bytes struct {
	PosInfo files.ExtraInfo
	Data    []byte
}

type Splitter interface {
	NextBytes() (Bytes, error)
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

			out <- b.Data
		}
	}()
	return out, errs
}

type sizeSplitterv2 struct {
	r    files.AdvReader
	size int64
	err  error
}

func NewSizeSplitter(r io.Reader, size int64) Splitter {
	return &sizeSplitterv2{
		r:    files.AdvReaderAdapter(r),
		size: size,
	}
}

func (ss *sizeSplitterv2) NextBytes() (Bytes, error) {
	posInfo := ss.r.ExtraInfo()
	if ss.err != nil {
		return Bytes{posInfo, nil}, ss.err
	}
	buf := make([]byte, ss.size)
	n, err := io.ReadFull(ss.r, buf)
	if err == io.ErrUnexpectedEOF {
		ss.err = io.EOF
		err = nil
	}
	if err != nil {
		return Bytes{posInfo, nil}, err
	}

	return Bytes{posInfo, buf[:n]}, nil
}
