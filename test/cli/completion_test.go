package cli

import (
	"fmt"
	"testing"

	"github.com/ipfs/go-ipfs/test/cli/harness"
)

func TestBashCompletion(t *testing.T) {
	h := harness.New(t)

	res := h.IPFS("commands", "completion", "bash")

	length := len(res.Stdout.String())
	if length < 100 {
		t.Fatalf("expected a long Bash completion file, but gone one of length %d", length)
	}

	completionFile := h.WriteToTemp(res.Stdout.String())
	h.Sh(fmt.Sprintf("source %s && type -t _ipfs", completionFile))
}
