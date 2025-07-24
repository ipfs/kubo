package cli

import (
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
)

func TestLogGetLevel(t *testing.T) {

	t.Run("get-level shows all subsystems", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// Get expected subsystem count from 'ipfs log ls'
		lsRes := node.IPFS("log", "ls")
		assert.NoError(t, lsRes.Err)
		expectedSubsystems := len(SplitLines(lsRes.Stdout.String()))

		res := node.IPFS("log", "get-level")
		assert.NoError(t, res.Err)
		assert.Equal(t, 0, len(res.Stderr.Lines()))

		output := res.Stdout.String()
		lines := SplitLines(output)

		// Should show all subsystems plus the global '*' level
		assert.GreaterOrEqual(t, len(lines), expectedSubsystems)

		// Check that each line has the format "subsystem: level"
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.Split(line, ": ")
			assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
			assert.NotEmpty(t, parts[0], "Subsystem should not be empty")
			assert.NotEmpty(t, parts[1], "Level should not be empty")
		}
	})

	t.Run("get-level with specific subsystem", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		node.IPFS("log", "level", "core", "debug")
		res := node.IPFS("log", "get-level", "core")
		assert.NoError(t, res.Err)
		assert.Equal(t, 0, len(res.Stderr.Lines()))

		output := res.Stdout.String()
		lines := SplitLines(output)

		assert.Equal(t, 1, len(lines))

		line := strings.TrimSpace(lines[0])
		parts := strings.Split(line, ": ")
		assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
		assert.Equal(t, "core", parts[0])
		assert.Equal(t, "debug", parts[1])
	})

	t.Run("get-level with 'all' returns global level", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		res1 := node.IPFS("log", "level", "all", "fatal")
		assert.NoError(t, res1.Err)
		assert.Equal(t, 0, len(res1.Stderr.Lines()))

		res := node.IPFS("log", "get-level", "all")
		assert.NoError(t, res.Err)
		assert.Equal(t, 0, len(res.Stderr.Lines()))

		output := res.Stdout.String()
		lines := SplitLines(output)

		assert.Equal(t, 1, len(lines))

		line := strings.TrimSpace(lines[0])
		parts := strings.Split(line, ": ")
		assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
		assert.Equal(t, "*", parts[0])
		assert.Equal(t, "fatal", parts[1])
	})

	t.Run("get-level with '*' returns global level", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		node.IPFS("log", "level", "*", "dpanic")
		res := node.IPFS("log", "get-level", "*")
		assert.NoError(t, res.Err)
		assert.Equal(t, 0, len(res.Stderr.Lines()))

		output := res.Stdout.String()
		lines := SplitLines(output)

		assert.Equal(t, 1, len(lines))

		line := strings.TrimSpace(lines[0])
		parts := strings.Split(line, ": ")
		assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
		assert.Equal(t, "*", parts[0])
		assert.Equal(t, "dpanic", parts[1])
	})

	t.Run("get-level reflects runtime log level changes", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
		defer node.StopDaemon()

		node.IPFS("log", "level", "core", "debug")
		res := node.IPFS("log", "get-level", "core")
		assert.NoError(t, res.Err)

		output := res.Stdout.String()
		lines := SplitLines(output)

		assert.Equal(t, 1, len(lines))

		line := strings.TrimSpace(lines[0])
		parts := strings.Split(line, ": ")
		assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
		assert.Equal(t, "core", parts[0])
		assert.Equal(t, "debug", parts[1])
	})

	t.Run("get-level with non-existent subsystem returns error", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		res := node.RunIPFS("log", "get-level", "non-existent-subsystem")
		assert.Error(t, res.Err)
		assert.NotEqual(t, 0, len(res.Stderr.Lines()))
	})

}
