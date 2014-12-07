package counting

import (
	"io"
)

// Reader wraps an existing io.Reader and keeps track of how much has been read
// from it.
type Reader struct {
	reader io.Reader
	count  int
}

// Read some bytes into b and return the number of bytes read and error if any.
func (r *Reader) Read(b []byte) (int, error) {
	n, err := r.reader.Read(b)
	r.count += n
	return n, err
}

// Count returns the number of bytes read so far.
func (r *Reader) Count() int {
	return r.count
}

// NewReader returns a new reader that will keep track of how much as been
// read.
func NewReader(r io.Reader) *Reader {
	return &Reader{reader: r}
}
