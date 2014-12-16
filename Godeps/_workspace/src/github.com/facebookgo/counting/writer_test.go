package counting_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/facebookgo/counting"
)

func TestWriter(t *testing.T) {
	const payload = "hello world"
	buf := bytes.NewBuffer(nil)
	countingW := counting.NewWriter(buf)
	fmt.Fprint(countingW, payload)
	if countingW.Count() != len(payload) {
		t.Fatalf("did not get expected count")
	}
	if buf.String() != payload {
		t.Fatalf("did not get expected payload: %s", buf.String())
	}
}
