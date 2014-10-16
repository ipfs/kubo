package version

import "testing"

func TestCompatible(t *testing.T) {
	tcases := []struct {
		a, b     *SemVer
		expected bool
	}{
		{NewSemVer(0, 0, 0), NewSemVer(0, 0, 0), true},
		{NewSemVer(0, 0, 0), NewSemVer(1, 0, 0), false},
		{NewSemVer(1, 0, 0), NewSemVer(0, 0, 0), false},
		{NewSemVer(1, 0, 0), NewSemVer(1, 0, 0), true},
	}

	for i, tcase := range tcases {
		if Compatible(tcase.a, tcase.b) != tcase.expected {
			t.Fatalf("case[%d] failed", i)
		}
	}
}
