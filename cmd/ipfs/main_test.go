package main

import (
	"testing"

	"gx/ipfs/QmVD1W3MC8Hk1WZgFQPWWmBECJ3X72BgUYf9eCQ4PGzPps/go-ipfs-cmdkit"
)

func TestIsCientErr(t *testing.T) {
	t.Log("Only catch pointers")
	if !isClientError(&cmdkit.Error{Code: cmdkit.ErrClient}) {
		t.Errorf("misidentified error")
	}
}
