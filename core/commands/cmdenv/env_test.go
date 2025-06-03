package cmdenv

import (
	"strconv"
	"testing"
)

func TestEscNonPrint(t *testing.T) {
	b := []byte("hello")
	b[2] = 0x7f
	s := string(b)
	if !needEscape(s) {
		t.Fatal("string needs escaping")
	}
	if !hasNonPrintable(s) {
		t.Fatal("expected non-printable")
	}
	if hasNonPrintable(EscNonPrint(s)) {
		t.Fatal("escaped string has non-printable")
	}
	if EscNonPrint(`hel\lo`) != `hel\\lo` {
		t.Fatal("backslash not escaped")
	}

	s = `hello`
	if needEscape(s) {
		t.Fatal("string does not need escaping")
	}
	if EscNonPrint(s) != s {
		t.Fatal("string should not have changed")
	}
	s = `"hello"`
	if EscNonPrint(s) != s {
		t.Fatal("string should not have changed")
	}
	if EscNonPrint(`"hel\"lo"`) != `"hel\\"lo"` {
		t.Fatal("did not get expected escaped string")
	}
}

func hasNonPrintable(s string) bool {
	for _, r := range s {
		if !strconv.IsPrint(r) {
			return true
		}
	}
	return false
}
