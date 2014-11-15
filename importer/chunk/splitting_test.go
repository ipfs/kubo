package chunk

import (
	"bytes"
	"crypto/rand"
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
