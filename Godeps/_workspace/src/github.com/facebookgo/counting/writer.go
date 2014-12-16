// Package counting provides wrappers to add counting to io.Reader and io.Writer.
package counting

import "io"

// Writer provides out of band access to the number of bytes written.
type Writer struct {
	writer io.Writer
	count  int
}

// NewWriter wraps an existing writer to track byte write count.
func NewWriter(writer io.Writer) *Writer {
	return &Writer{writer: writer}
}

// Write proxies to the underlying writer.
func (h *Writer) Write(p []byte) (n int, err error) {
	n, err = h.writer.Write(p)
	h.count += n
	return
}

// Count returns the total number of bytes that were written to the underlying
// writer since the last call to Clear.
func (h *Writer) Count() int {
	return h.count
}

// Clear zeroes the byte write counter.
func (h *Writer) Clear() {
	h.count = 0
}
