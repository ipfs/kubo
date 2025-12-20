package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
	// Use Eventually to handle async bitswap timing - the peer connection
	// may need a moment before bitswap can successfully retrieve blocks.
	require.Eventually(t, func() bool {
		cmd := exec.CommandContext(t.Context(), "go", "run", "main.go")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("attempt failed: %v\nOutput:\n%s", err, out)
			return false
		}
		if !strings.Contains(string(out), "All done!") {
			t.Logf("example did not complete successfully\nOutput:\n%s", out)
			return false
		}
		return true
	}, 10*time.Minute, 5*time.Second, "example failed to complete successfully")
}
