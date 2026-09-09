package cli

import (
	"fmt"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
)

func TestBashCompletion(t *testing.T) {
	t.Parallel()
	h := harness.NewT(t)
	node := h.NewNode()

	res := node.IPFS("commands", "completion", "bash")

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

func TestZshCompletion(t *testing.T) {
	t.Parallel()
	h := harness.NewT(t)
	node := h.NewNode()

	res := node.IPFS("commands", "completion", "zsh")

	length := len(res.Stdout.String())
	if length < 100 {
		t.Fatalf("expected a long Bash completion file, but got one of length %d", length)
	}

	t.Run("completion file can be loaded in bash", func(t *testing.T) {
		RequiresLinux(t)

		completionFile := h.WriteToTemp(res.Stdout.String())
		res = h.Runner.Run(harness.RunRequest{
			Path: "zsh",
			// compinit -i skips insecure (group/other-writable) fpath dirs instead of
			// prompting about them; the prompt aborts in non-interactive shells (no tty),
			// e.g. on GitHub-hosted runners. This test validates kubo's generated
			// completion script, not the host's fpath hygiene.
			Args: []string{"-c", fmt.Sprintf("autoload -Uz compinit && compinit -i && source %s && echo -E $_comps[ipfs]", completionFile)},
		})

		assert.NoError(t, res.Err)
		assert.NotEmpty(t, res.Stdout.String())
	})
}
