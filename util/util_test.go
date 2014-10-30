package util

import (
	"bytes"
	"math/rand"
	"testing"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

func TestKey(t *testing.T) {

	h1, err := mh.Sum([]byte("beep boop"), mh.SHA2_256, -1)
	if err != nil {
		t.Error(err)
	}

	k1 := Key(h1)
	h2 := mh.Multihash(k1)
	k2 := Key(h2)

	if !bytes.Equal(h1, h2) {
		t.Error("Multihashes not equal.")
	}

	if k1 != k2 {
		t.Error("Keys not equal.")
	}
}

func TestByteChanReader(t *testing.T) {

	var data bytes.Buffer
	var data2 bytes.Buffer
	dch := make(chan []byte, 8)
	randr := NewTimeSeededRand()

	go func() {
		defer close(dch)
		for i := 0; i < rand.Intn(100)+100; i++ {
			chunk := make([]byte, rand.Intn(100000)+10)
			randr.Read(chunk)
			data.Write(chunk)
			// fmt.Printf("chunk: %6.d %v\n", len(chunk), chunk[:10])
			dch <- chunk
		}
	}()

	read := NewByteChanReader(dch)

	// read in random, weird sizes to exercise saving buffer.
	for {
		buf := make([]byte, rand.Intn(10)*10)
		n, err := read.Read(buf)
		data2.Write(buf[:n])
		// fmt.Printf("read: %6.d\n", n)
		if err != nil {
			break
		}
	}

	// fmt.Printf("lens: %d == %d\n", len(out), len(data.Bytes()))
	if !bytes.Equal(data2.Bytes(), data.Bytes()) {
		t.Fatal("Reader failed to stream correct bytes")
	}
}

func TestXOR(t *testing.T) {
	cases := [][3][]byte{
		[3][]byte{
			[]byte{0xFF, 0xFF, 0xFF},
			[]byte{0xFF, 0xFF, 0xFF},
			[]byte{0x00, 0x00, 0x00},
		},
		[3][]byte{
			[]byte{0x00, 0xFF, 0x00},
			[]byte{0xFF, 0xFF, 0xFF},
			[]byte{0xFF, 0x00, 0xFF},
		},
		[3][]byte{
			[]byte{0x55, 0x55, 0x55},
			[]byte{0x55, 0xFF, 0xAA},
			[]byte{0x00, 0xAA, 0xFF},
		},
	}

	for _, c := range cases {
		r := XOR(c[0], c[1])
		if !bytes.Equal(r, c[2]) {
			t.Error("XOR failed")
		}
	}
}
