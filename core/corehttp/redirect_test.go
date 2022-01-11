package corehttp

import (
	"fmt"
	"testing"
)

func TestRedirline(t *testing.T) {
	for _, tc := range []struct {
		matcher string
		s       string
		exp     bool
		errExp  bool
	}{
		{"hi", "hi", true, false},
		{"hi", "hithere", true, false},
		{"^hi$", "hithere", false, false},
		{"^hi$", "hi", true, false},
		{"hi.*", "hithere", true, false},
		{"/hi", "/hi/there", true, false},
		{"^/hi/", "/hi/there/now", true, false},
		{"^/hi/", "/hithere", false, false},
		{"^/hi/(.*", "/hi/there/now", false, true},
	} {
		r := redirLine{tc.matcher, "to", 200}
		ok, err := r.match(tc.s)
		if ok != tc.exp {
			t.Errorf("%v %v, expected %v, got %v", tc.matcher, tc.s, tc.exp,
				ok)
		}

		if err != nil != tc.errExp {
			fmt.Printf("regexp error %v\n", err)
			t.Errorf("%v %v, expected error %v, got %v", tc.matcher, tc.s, tc.errExp, err == nil)
		}
	}
}
