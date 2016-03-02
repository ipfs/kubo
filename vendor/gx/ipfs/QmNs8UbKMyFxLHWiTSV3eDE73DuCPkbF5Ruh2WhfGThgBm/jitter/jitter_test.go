package jitter

import "testing"

func TestZeroDuration(t *testing.T) {
	if Duration(0, 0) != 0 {
		t.FailNow()
	}
}
