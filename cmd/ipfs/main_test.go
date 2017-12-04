package main

import (
	"testing"

	"gx/ipfs/QmQp2a2Hhb7F6eK2A5hN8f9aJy4mtkEikL9Zj4cgB7d1dD/go-ipfs-cmdkit"
)

func TestIsCientErr(t *testing.T) {
	t.Log("Only catch pointers")
	if !isClientError(&cmdkit.Error{Code: cmdkit.ErrClient}) {
		t.Errorf("misidentified error")
	}
}
