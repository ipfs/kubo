package util

import (
	"bytes"
	"testing"

	mh "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
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

func BenchmarkHash256K(b *testing.B) {
	buf := make([]byte, 256*1024)
	NewTimeSeededRand().Read(buf)
	b.SetBytes(int64(256 * 1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Hash(buf)
	}
}

func BenchmarkHash512K(b *testing.B) {
	buf := make([]byte, 512*1024)
	NewTimeSeededRand().Read(buf)
	b.SetBytes(int64(512 * 1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Hash(buf)
	}
}

func BenchmarkHash1M(b *testing.B) {
	buf := make([]byte, 1024*1024)
	NewTimeSeededRand().Read(buf)
	b.SetBytes(int64(1024 * 1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Hash(buf)
	}
}
