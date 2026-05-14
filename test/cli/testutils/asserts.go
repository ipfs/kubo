package testutils

import (
	"strings"
	"testing"
)

func AssertStringContainsOneOf(t *testing.T, str string, ss ...string) {
	for _, s := range ss {
		if strings.Contains(str, s) {
			return
		}
	}
	t.Errorf("%q does not contain one of %v", str, ss)
}
