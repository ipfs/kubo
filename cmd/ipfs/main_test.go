package main

import (
	"testing"

	"gx/ipfs/QmUyfy4QSr3NXym4etEiRyxBLqqAeKHJuRdi8AACxg63fZ/go-ipfs-cmdkit"
)

func TestIsCientErr(t *testing.T) {
	t.Log("Only catch pointers")
	if !isClientError(&cmdkit.Error{Code: cmdkit.ErrClient}) {
		t.Errorf("misidentified error")
	}
}
