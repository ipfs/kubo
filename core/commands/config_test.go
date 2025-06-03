package commands

import "testing"

func TestScrubMapInternalDelete(t *testing.T) {
	m, err := scrubMapInternal(nil, nil, true)
	if err != nil {
		t.Error(err)
	}
	if m == nil {
		t.Errorf("expecting an empty map, got nil")
	}
	if len(m) != 0 {
		t.Errorf("expecting an empty map, got a non-empty map")
	}
}
