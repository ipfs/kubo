package io

import (
	"bytes"
	"io"
	"testing"
)

func testSizeAdjRead(t *testing.T, gen func() (*sizeAdjReadSeekCloser, []byte)) {
	r, expected := gen()
	actual := make([]byte, len(expected)+1) // longer then necessary on purpose
	n, err := r.Read(actual)
	if err != nil && err != io.EOF {
		t.Errorf("write failed: %v", err)
		return
	}
	if n != len(expected) {
		t.Errorf("n != len(expected); %d != %d", n, len(expected))
	}
}

func testSizeAdjWriteTo(t *testing.T, gen func() (*sizeAdjReadSeekCloser, []byte)) {
	r, expected := gen()
	buf := new(bytes.Buffer)
	n, err := r.WriteTo(buf)
	//actual := buf.Bytes()
	if err != nil {
		t.Errorf("write failed: %v", err)
		return
	}
	if n != int64(len(expected)) {
		t.Errorf("n != len(expected); %d != %d", n, len(expected))
	}
}

func testBytes() []byte {
	b := make([]byte, 128)
	for i := 0; i < 100; i++ {
		b[i] = byte(i + 1)
	}
	return b
}

// byteReader is bytes.Reader with a noop Close() method
type byteReader struct {
	*bytes.Reader
}

func (byteReader) Close() error { return nil }

func simpleSizeAdj() (*sizeAdjReadSeekCloser, []byte) {
	b := testBytes()
	buf := byteReader{bytes.NewReader(b)}
	return newSizeAdjReadSeekCloser(buf, uint64(len(b))), b
}

func truncSizeAdj() (*sizeAdjReadSeekCloser, []byte) {
	b := testBytes()
	buf := byteReader{bytes.NewReader(b)}
	return newSizeAdjReadSeekCloser(buf, 100), b[:100]
}

func padSizeAdj() (*sizeAdjReadSeekCloser, []byte) {
	b := testBytes()
	buf := byteReader{bytes.NewReader(b[:100])}
	return newSizeAdjReadSeekCloser(buf, uint64(len(b))), b
}

func TestSizeAdj(t *testing.T) {
	t.Run("Read/Simple", func(t *testing.T) { testSizeAdjRead(t, simpleSizeAdj) })
	t.Run("Read/Trunc", func(t *testing.T) { testSizeAdjRead(t, truncSizeAdj) })
	t.Run("Read/Pad", func(t *testing.T) { testSizeAdjRead(t, padSizeAdj) })
	t.Run("WriteTo/Simple", func(t *testing.T) { testSizeAdjWriteTo(t, simpleSizeAdj) })
	t.Run("WriteTo/Trunc", func(t *testing.T) { testSizeAdjWriteTo(t, truncSizeAdj) })
	t.Run("WriteTo/Pad", func(t *testing.T) { testSizeAdjWriteTo(t, padSizeAdj) })
}
