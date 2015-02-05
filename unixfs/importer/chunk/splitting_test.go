package chunk

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"
)

func randBuf(t *testing.T, size int) []byte {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal("failed to read enough randomness")
	}
	return buf
}

func copyBuf(buf []byte) []byte {
	cpy := make([]byte, len(buf))
	copy(cpy, buf)
	return cpy
}

func TestSizeSplitterIsDeterministic(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	test := func() {
		bufR := randBuf(t, 10000000) // crank this up to satisfy yourself.
		bufA := copyBuf(bufR)
		bufB := copyBuf(bufR)

		chunksA := DefaultSplitter.Split(bytes.NewReader(bufA))
		chunksB := DefaultSplitter.Split(bytes.NewReader(bufB))

		for n := 0; ; n++ {
			a, moreA := <-chunksA
			b, moreB := <-chunksB

			if !moreA {
				if moreB {
					t.Fatal("A ended, B didnt.")
				}
				return
			}

			if !bytes.Equal(a, b) {
				t.Fatalf("chunk %d not equal", n)
			}
		}
	}

	for run := 0; run < 1; run++ { // crank this up to satisfy yourself.
		test()
	}
}

func TestSizeSplitterFillsChunks(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	max := 10000000
	b := randBuf(t, max)
	r := &clipReader{r: bytes.NewReader(b), size: 4000}
	s := SizeSplitter{Size: 1024 * 256}
	c := s.Split(r)

	sofar := 0
	whole := make([]byte, max)
	for chunk := range c {

		bc := b[sofar : sofar+len(chunk)]
		if !bytes.Equal(bc, chunk) {
			t.Fatalf("chunk not correct: (sofar: %d) %d != %d, %v != %v", sofar, len(bc), len(chunk), bc[:100], chunk[:100])
		}

		copy(whole[sofar:], chunk)

		sofar += len(chunk)
		if sofar != max && len(chunk) < s.Size {
			t.Fatal("sizesplitter split at a smaller size")
		}
	}

	if !bytes.Equal(b, whole) {
		t.Fatal("splitter did not split right")
	}
}

type clipReader struct {
	size int
	r    io.Reader
}

func (s *clipReader) Read(buf []byte) (int, error) {

	// clip the incoming buffer to produce smaller chunks
	if len(buf) > s.size {
		buf = buf[:s.size]
	}

	return s.r.Read(buf)
}
