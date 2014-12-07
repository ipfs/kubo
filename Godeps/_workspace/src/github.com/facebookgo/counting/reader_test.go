package counting_test

import (
	"strings"
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/facebookgo/counting"
)

func TestReader(t *testing.T) {
	buf := strings.NewReader("123")
	countingR := counting.NewReader(buf)
	out := make([]byte, 4)
	n, err := countingR.Read(out)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("expected 3 got %d", n)
	}
	if countingR.Count() != 3 {
		t.Fatalf("expected 3 got %d", countingR.Count())
	}
}
