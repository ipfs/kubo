package infduration

import (
	"testing"
	"time"
)

var durations = [...]time.Duration{
	-10000 * time.Hour,
	-1,
	0,
	1,
	10 * time.Second,
	10000 * time.Hour,
}

func TestInfinite(t *testing.T) {
	dur := InfiniteDuration()
	if b := IsInfinite(dur); !b {
		t.Errorf("IsInfinite(%v): got %v, expected %v", dur, b, true)
	}
	if got := GetFiniteDuration(dur, 42); got != 42 {
		t.Errorf("GetFiniteDuration(%v, 42): got %v, expected %v", dur, got, 42)
	}
}

func TestFinite(t *testing.T) {
	for _, tdur := range durations {
		dur := FiniteDuration(tdur)
		if b := IsInfinite(dur); b {
			t.Errorf("IsInfinite(%v): got %v, expected %v", dur, b, false)
		}
		if got := GetFiniteDuration(dur, 42); got != tdur {
			t.Errorf("GetFiniteDuration(%v, 42): got %v, expected %v", dur, got, tdur)
		}
	}
}

func TestEqualMinMax(t *testing.T) {
	inf := InfiniteDuration()

	if b := Equal(inf, inf); !b {
		t.Errorf("Equal(%v, %v): got %v, expected %v", inf, inf, b, true)
	}
	if got := Min(inf, inf); !Equal(got, inf) {
		t.Errorf("Min(%v, %v): got %v, expected %v", inf, inf, got, inf)
	}
	if got := Max(inf, inf); !Equal(got, inf) {
		t.Errorf("Max(%v, %v): got %v, expected %v", inf, inf, got, inf)
	}

	for _, tdur := range durations {
		dur := FiniteDuration(tdur)
		if b := Equal(dur, inf); b {
			t.Errorf("Equal(%v, %v): got %v, expected %v", dur, inf, b, false)
		}
		if b := Equal(inf, dur); b {
			t.Errorf("Equal(%v, %v): got %v, expected %v", inf, dur, b, false)
		}
		if got := Min(dur, inf); !Equal(got, dur) {
			t.Errorf("Min(%v, %v): got %v, expected %v", dur, inf, got, dur)
		}
		if got := Min(inf, dur); !Equal(got, dur) {
			t.Errorf("Min(%v, %v): got %v, expected %v", inf, dur, got, dur)
		}
		if got := Max(dur, inf); !Equal(got, inf) {
			t.Errorf("Max(%v, %v): got %v, expected %v", dur, inf, got, inf)
		}
		if got := Max(inf, dur); !Equal(got, inf) {
			t.Errorf("Max(%v, %v): got %v, expected %v", inf, dur, got, inf)
		}
	}

	for _, tdurA := range durations {
		durA := FiniteDuration(tdurA)
		for _, tdurB := range durations {
			durB := FiniteDuration(tdurB)

			durMin, durMax := durA, durB
			if tdurB < tdurA {
				durMin, durMax = durB, durA
			}
			eq := tdurA == tdurB

			if b := Equal(durA, durB); b != eq {
				t.Errorf("Equal(%v, %v): got %v, expected %v", durA, durB, b, eq)
			}
			if got := Min(durA, durB); !Equal(got, durMin) {
				t.Errorf("Min(%v, %v): got %v, expected %v", durA, durB, got, durMin)
			}
			if got := Max(durA, durB); !Equal(got, durMax) {
				t.Errorf("Max(%v, %v): got %v, expected %v", durA, durB, got, durMax)
			}
		}
	}
}
