package util

import (
	"bytes"
	"io/ioutil"
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
	data := make([]byte, 1024*1024)
	r := NewFastRand()
	r.Read(data)
	dch := make(chan []byte, 8)

	go func() {
		beg := 0
		for i := 0; i < len(data); {
			i += rand.Intn(100) + 1
			if i > len(data) {
				i = len(data)
			}
			dch <- data[beg:i]
			beg = i
		}
		close(dch)
	}()

	read := NewByteChanReader(dch)
	out, err := ioutil.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, data) {
		t.Fatal("Reader failed to stream correct bytes")
	}
}
