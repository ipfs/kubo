// package chunk implements streaming block splitters
package chunk

import (
	"io"

	"github.com/ipfs/go-ipfs/commands/files"
	logging "gx/ipfs/Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH/go-log"
)

var log = logging.Logger("chunk")

var DefaultBlockSize int64 = 1024 * 256

type Splitter interface {
	// returns the data, an offset if applicable and, an error condition
	NextBytes() ([]byte, int64, error)
	// returns the full path to the file if applicable
	FilePath() string
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
			b, _, err := s.NextBytes()
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

func (ss *sizeSplitterv2) NextBytes() ([]byte, int64, error) {
	if ss.err != nil {
		return nil, -1, ss.err
	}
	buf := make([]byte, ss.size)
	offset := ss.r.Offset()
	n, err := io.ReadFull(ss.r, buf)
	if err == io.ErrUnexpectedEOF {
		ss.err = io.EOF
		err = nil
	}
	if err != nil {
		return nil, -1, err
	}

	return buf[:n], offset, nil
}

func (ss *sizeSplitterv2) FilePath() string {
	return ss.r.FullPath()
}
