package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestInitPerms(t *testing.T) {
	t.Parallel()
	h := harness.NewT(t)
	badDir := filepath.Join(h.Dir, ".badipfs")
	h.Runner.Env["IPFS_PATH"] = badDir

	err := os.Mkdir(badDir, 0000)
	assert.NoError(t, err)

	res := h.Runner.Run(harness.RunRequest{
		Path: h.IPFSBin,
		Args: []string{"init"},
	})
	assert.NotEqual(t, 0, res.Cmd.ProcessState.ExitCode())
	assert.Contains(t, res.Stderr.String(), "permission denied")
}
