package io

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func testSizeAdjRead(t *testing.T, gen func() (*sizeAdjReadSeekCloser, []byte)) {
	r, expected := gen()
	actual, err := ioutil.ReadAll(r)
	if err != nil {
		t.Errorf("write failed: %v", err)
		return
	}
	if len(actual) != len(expected) {
		t.Errorf("len(actual) != len(expected); %d != %d", len(actual), len(expected))
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
	r, _ := newSizeAdjReadSeekCloser(buf, uint64(len(b)))
	return r, b
}

func truncSizeAdj() (*sizeAdjReadSeekCloser, []byte) {
	b := testBytes()
	buf := byteReader{bytes.NewReader(b)}
	r, _ := newSizeAdjReadSeekCloser(buf, 100)
	return r, b[:100]
}

func padSizeAdj() (*sizeAdjReadSeekCloser, []byte) {
	b := testBytes()
	buf := byteReader{bytes.NewReader(b[:100])}
	r, _ := newSizeAdjReadSeekCloser(buf, uint64(len(b)))
	return r, b
}

func TestSizeAdj(t *testing.T) {
	t.Run("Read/Simple", func(t *testing.T) { testSizeAdjRead(t, simpleSizeAdj) })
	t.Run("Read/Trunc", func(t *testing.T) { testSizeAdjRead(t, truncSizeAdj) })
	t.Run("Read/Pad", func(t *testing.T) { testSizeAdjRead(t, padSizeAdj) })
	t.Run("WriteTo/Simple", func(t *testing.T) { testSizeAdjWriteTo(t, simpleSizeAdj) })
	t.Run("WriteTo/Trunc", func(t *testing.T) { testSizeAdjWriteTo(t, truncSizeAdj) })
	t.Run("WriteTo/Pad", func(t *testing.T) { testSizeAdjWriteTo(t, padSizeAdj) })
}

func TestTruncWriter(t *testing.T) {
	b := testBytes() // 128 bytes
	buf := new(bytes.Buffer)
	// note: to correctly test truncWriter size should not be a
	//   multiple of writeLen
	tw := truncWriter{base: buf, size: 45}
	writeLen := 25
	inCount := 0
	for len(b) > 0 {
		j := writeLen
		if j > len(b) {
			j = len(b)
		}
		n, err := tw.Write(b[:j])
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
		b = b[j:]
		inCount += n
	}
	res := buf.Bytes()
	if inCount != 128 {
		t.Fatalf("truncWriter accepted incorrect number of bytes (expected 128): %d", len(res))
	}
	if len(res) != 45 {
		t.Fatalf("truncWriter wrote incorrect number of bytes (expected 45): %d", len(res))
	}
}

func TestWriteZeros(t *testing.T) {
	buf := new(bytes.Buffer)

	n, err := writeZeros(buf, 1000)
	if err != nil {
		t.Fatalf("writeZeros failed: %v", err)
	}
	res := buf.Bytes()
	if n != 1000 || len(res) != 1000 {
		t.Fatalf("writeZeros wrote incorrect number of bytes (expected 1000): %d %d", n, len(res))
	}

	buf.Reset()
	n, err = writeZeros(buf, 10000)
	if err != nil {
		t.Fatalf("writeZeros failed: %v", err)
	}
	res = buf.Bytes()
	if n != 10000 || len(res) != 10000 {
		t.Fatalf("writeZeros wrote incorrect number of bytes (expected 10000): %d %d", n, len(res))
	}
}
