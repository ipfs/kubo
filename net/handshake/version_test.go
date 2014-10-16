package handshake

import "testing"

func TestCompatible(t *testing.T) {
	tcases := []struct {
		a, b     string
		expected error
	}{
		{"0.0.0", "0.0.0", nil},
		{"1.0.0", "1.1.0", nil},
		{"1.0.0", "1.0.1", nil},
		{"0.0.0", "1.0.0", ErrVersionMismatch},
		{"1.0.0", "0.0.0", ErrVersionMismatch},
	}

	for i, tcase := range tcases {

		if Compatible(NewHandshake1(tcase.a, ""), NewHandshake1(tcase.b, "")) != tcase.expected {
			t.Fatalf("case[%d] failed", i)
		}
	}
}
