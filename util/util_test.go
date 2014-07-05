package util

import (
  "bytes"
  "fmt"
  mh "github.com/jbenet/go-multihash"
  "testing"
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
