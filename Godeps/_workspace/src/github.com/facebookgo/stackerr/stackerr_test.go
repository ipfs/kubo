package stackerr_test

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/facebookgo/stackerr"
)

func TestNew(t *testing.T) {
	const errStr = "foo bar baz"
	e := stackerr.New(errStr)
	matches := []string{
		errStr,
		"^github.com/facebookgo/stackerr/stackerr_test.go:15 +TestNew$",
	}
	match(t, e.Error(), matches)
}

func TestNewf(t *testing.T) {
	const fmtStr = "%s 42"
	const errStr = "foo bar baz"
	e := stackerr.Newf(fmtStr, errStr)
	matches := []string{
		fmt.Sprintf(fmtStr, errStr),
		"^github.com/facebookgo/stackerr/stackerr_test.go:26 +TestNewf$",
	}
	match(t, e.Error(), matches)
}

func TestWrap(t *testing.T) {
	const errStr = "foo bar baz"
	e := stackerr.Wrap(errors.New(errStr))
	matches := []string{
		errStr,
		"^github.com/facebookgo/stackerr/stackerr_test.go:36 +TestWrap$",
	}
	match(t, e.Error(), matches)
}

func TestNilWrap(t *testing.T) {
	if stackerr.WrapSkip(nil, 1) != nil {
		t.Fatal("did not get nil error")
	}
}

func TestDoubleWrap(t *testing.T) {
	e := stackerr.New("")
	if stackerr.WrapSkip(e, 1) != e {
		t.Fatal("double wrap failure")
	}
}

func TestLog(t *testing.T) {
	t.Log(stackerr.New("hello"))
}

func TestUnderlying(t *testing.T) {
	e1 := errors.New("")
	e2 := stackerr.Wrap(e1)
	errs := stackerr.Underlying(e2)
	if len(errs) != 2 || errs[0] != e2 || errs[1] != e1 {
		t.Fatal("failed Underlying")
	}
}

func match(t testing.TB, s string, matches []string) {
	lines := strings.Split(s, "\n")
	for i, m := range matches {
		if !regexp.MustCompile(m).MatchString(lines[i]) {
			t.Fatalf(
				"did not find expected match \"%s\" on line %d in:\n%s",
				m,
				i,
				s,
			)
		}
	}
}
