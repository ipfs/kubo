package cli

import (
	"os/exec"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
)

func TestDaemon(t *testing.T) {
	t.Parallel()

	t.Run("daemon starts if api is set to null", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Addresses.API", nil)
		node.Runner.MustRun(harness.RunRequest{
			Path:    node.IPFSBin,
			Args:    []string{"daemon"},
			RunFunc: (*exec.Cmd).Start, // Start without waiting for completion.
		})

		node.StopDaemon()
	})
}
