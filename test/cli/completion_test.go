package cli

import (
	"fmt"
	"testing"

	"github.com/ipfs/go-ipfs/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestBashCompletion(t *testing.T) {
	h := harness.NewForTest(t)

	res := h.MustRunIPFS("commands", "completion", "bash")

	length := len(res.Stdout.String())
	if length < 100 {
		t.Fatalf("expected a long Bash completion file, but got one of length %d", length)
	}

	t.Run("completion file can be loaded in bash", func(t *testing.T) {
		RequiresLinux(t)
		completionFile := h.WriteToTemp(res.Stdout.String())
		res = h.Sh(fmt.Sprintf("source %s && type -t _ipfs", completionFile))
		assert.NoError(t, res.Err)
	})
}
