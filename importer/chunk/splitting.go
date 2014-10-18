package chunk

import (
	"io"

	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("chunk")

var DefaultSplitter = &SizeSplitter{size: 1024 * 512}

// BlockSplitter is the interface to a block splitting algorithm.
type BlockSplitter interface {

	// Size returns the maximum block size that this BlockSplitter may produce,
	// or the maximum amount of data the BlockSplitter may buffer,
	// whichever is larger.
	Size() int

	// Next returns a block split from the underlying reader.
	// io.EOF is returned when the both last Reader and any splitting buffers
	// are exausted.
	Next() ([]byte, error)

	// Push causes the Reader to start reading from a new io.Reader.
	// When an EOF error is seen from the new io.Reader, it is popped
	// and the Reader continues to read from the next most recent io.Reader.
	Push(io.Reader)
}

type SizeSplitter struct {
	size    int
	readers []io.Reader
}

func (ss *SizeSplitter) Size() int { return ss.size }

func (ss *SizeSplitter) Next() (b []byte, err error) {
	b = make([]byte, ss.size)

	var n, N, ri int
	for len(ss.readers) > 0 {
		ri = len(ss.readers) - 1
		N, err = ss.readers[ri].Read(b[n:])
		n += N
		if err == io.EOF {
			ss.readers = ss.readers[:ri]
			err = nil
		}
		if n == ss.size {
			return
		}
	}
	if n == 0 {
		return nil, io.EOF
	}
	b = b[:n]
	return
}

func (ss *SizeSplitter) Push(r io.Reader) {
	ss.readers = append(ss.readers, r)
}

func NewSizeSplitter(size int) *SizeSplitter { return &SizeSplitter{size: size} }
