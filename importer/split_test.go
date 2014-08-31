package importer

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestDataSplitting(t *testing.T) {
	buf := make([]byte, 16*1024*1024)
	rand.Read(buf)
	split := Rabin(buf)

	if len(split) == 1 {
		t.Fatal("No split occurred!")
	}

	min := 2 << 15
	max := 0

	mxcount := 0

	n := 0
	for _, b := range split {
		if !bytes.Equal(b, buf[n:n+len(b)]) {
			t.Fatal("Split lost data!")
		}
		n += len(b)

		if len(b) < min {
			min = len(b)
		}

		if len(b) > max {
			max = len(b)
		}

		if len(b) == 16384 {
			mxcount++
		}
	}

	if n != len(buf) {
		t.Fatal("missing some bytes!")
	}
	t.Log(len(split))
	t.Log(min, max, mxcount)
}
