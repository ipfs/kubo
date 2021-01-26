package commands

import "testing"

func TestScrubMapInternalDelete(t *testing.T) {
	m, err := scrubMapInternal(nil, nil, true)
	if err != nil {
		t.Error(err)
	}
	if _, ok := m.(map[string]interface{}); !ok {
		t.Errorf("expecting an empty map, got nil")
	}
}
