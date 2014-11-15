package stack_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/facebookgo/stack"
)

func indirect1() stack.Stack {
	return stack.Callers(0)
}

func indirect2() stack.Stack {
	return indirect1()
}

func indirect3() stack.Stack {
	return indirect2()
}

func TestCallers(t *testing.T) {
	s := indirect3()
	matches := []string{
		"^github.com/facebookgo/stack/stack_test.go:12 +indirect1$",
		"^github.com/facebookgo/stack/stack_test.go:16 +indirect2$",
		"^github.com/facebookgo/stack/stack_test.go:20 +indirect3$",
		"^github.com/facebookgo/stack/stack_test.go:24 +TestCallers$",
	}
	match(t, s.String(), matches)
}

func TestCallersMulti(t *testing.T) {
	m := stack.CallersMulti(0)
	const expected = "github.com/facebookgo/stack/stack_test.go:35 TestCallersMulti"
	first := m.Stacks()[0][0].String()
	if first != expected {
		t.Fatalf(`expected "%s" got "%s"`, expected, first)
	}
}

func TestCallersMultiWithTwo(t *testing.T) {
	m := stack.CallersMulti(0)
	m.AddCallers(0)
	matches := []string{
		"^github.com/facebookgo/stack/stack_test.go:44 +TestCallersMultiWithTwo$",
		"",
		"",
		`^\(Stack 2\)$`,
		"^github.com/facebookgo/stack/stack_test.go:46 +TestCallersMultiWithTwo$",
	}
	match(t, m.String(), matches)
}

type typ struct{}

func (m typ) indirect1() stack.Stack {
	return stack.Callers(0)
}

func (m typ) indirect2() stack.Stack {
	return m.indirect1()
}

func (m typ) indirect3() stack.Stack {
	return m.indirect2()
}

func TestCallersWithStruct(t *testing.T) {
	var m typ
	s := m.indirect3()
	matches := []string{
		"^github.com/facebookgo/stack/stack_test.go:59 +typ.indirect1$",
		"^github.com/facebookgo/stack/stack_test.go:63 +typ.indirect2$",
		"^github.com/facebookgo/stack/stack_test.go:67 +typ.indirect3$",
		"^github.com/facebookgo/stack/stack_test.go:72 +TestCallersWithStruct$",
	}
	match(t, s.String(), matches)
}

func TestCaller(t *testing.T) {
	f := stack.Caller(0)
	const expected = "github.com/facebookgo/stack/stack_test.go:83 TestCaller"
	if f.String() != expected {
		t.Fatalf(`expected "%s" got "%s"`, expected, f)
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
