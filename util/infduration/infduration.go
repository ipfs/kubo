package infduration

import (
	"fmt"
	"time"
)

// Duration represents a possibly (positive) infinite time.Duration.
type Duration struct {
	// A positive infinite duration is internally represented as a nil
	// pointer, but this implementation detail is not to be exposed.
	tduration *time.Duration
}

// InfiniteDuration constructs a positive infinite Duration.
func InfiniteDuration() Duration {
	return Duration{tduration: nil}
}

// FiniteDuration constructs a finite Duration given a time.Duration.
func FiniteDuration(d time.Duration) Duration {
	return Duration{tduration: &d}
}

func (dur Duration) String() string {
	if dur.tduration != nil {
		return fmt.Sprintf("FiniteDuration(%v)", *dur.tduration)
	}
	return "InfiniteDuration()"
}

// IsInfinite indicates whether a given Duration is infinite.
func IsInfinite(dur Duration) bool {
	return dur.tduration == nil
}

// GetFiniteDuration returns the time.Duration held by an Duration if it is
// finite, falling back to the parameter otherwise.
func GetFiniteDuration(dur Duration, infinity time.Duration) (tdur time.Duration) {
	if dur.tduration != nil {
		return *dur.tduration
	}
	return infinity
}

// Equal returns true if both Durations are infinite or both are finite and
// equal.
func Equal(durA, durB Duration) bool {
	if durA.tduration == nil && durB.tduration == nil {
		return true
	}
	if durA.tduration == nil || durB.tduration == nil {
		return false
	}
	return *durA.tduration == *durB.tduration
}

// Min returns the lesser Duration.
func Min(idufA, idufB Duration) Duration {
	if IsInfinite(idufB) {
		return idufA
	}
	if IsInfinite(idufA) {
		return idufB
	}
	if GetFiniteDuration(idufA, 0) <= GetFiniteDuration(idufB, 0) {
		return idufA
	}
	return idufB
}

// Max returns the greater Duration.
func Max(idufA, idufB Duration) Duration {
	if IsInfinite(idufA) {
		return idufA
	}
	if IsInfinite(idufB) {
		return idufB
	}
	if GetFiniteDuration(idufA, 0) >= GetFiniteDuration(idufB, 0) {
		return idufA
	}
	return idufB
}
