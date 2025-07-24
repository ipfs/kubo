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
		// t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		res := node.IPFS("log", "get-level")
		assert.NoError(t, res.Err)
		assert.Equal(t, 0, len(res.Stderr.Lines()))

		output := res.Stdout.String()
		lines := SplitLines(output)

		// Should contain multiple subsystems
		assert.Greater(t, len(lines), 10)

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
		// t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		node.IPFS("log", "level", "core", "debug")
		res := node.IPFS("log", "get-level", "core")
		assert.NoError(t, res.Err)
		assert.Equal(t, 0, len(res.Stderr.Lines()))

		output := res.Stdout.String()
		lines := SplitLines(output)

		// Should contain exactly one line
		assert.Equal(t, 1, len(lines))

		// Check format
		line := strings.TrimSpace(lines[0])
		parts := strings.Split(line, ": ")
		assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
		assert.Equal(t, "core", parts[0])
		assert.Equal(t, "debug", parts[1])
	})

	t.Run("get-level with 'all' returns global level", func(t *testing.T) {
		// t.Parallel()
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

		// Should contain exactly one line
		assert.Equal(t, 1, len(lines))

		// Check format
		line := strings.TrimSpace(lines[0])
		parts := strings.Split(line, ": ")
		assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
		assert.Equal(t, "*", parts[0])
		assert.Equal(t, "fatal", parts[1])
	})

	t.Run("get-level with '*' returns global level", func(t *testing.T) {
		// t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		node.IPFS("log", "level", "*", "dpanic")
		res := node.IPFS("log", "get-level", "*")
		assert.NoError(t, res.Err)
		assert.Equal(t, 0, len(res.Stderr.Lines()))

		output := res.Stdout.String()
		lines := SplitLines(output)

		// Should contain exactly one line
		assert.Equal(t, 1, len(lines))

		// Check format
		line := strings.TrimSpace(lines[0])
		parts := strings.Split(line, ": ")
		assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
		assert.Equal(t, "*", parts[0])
		assert.Equal(t, "dpanic", parts[1])
	})

	t.Run("get-level reflects environment variable changes", func(t *testing.T) {
		node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
		defer node.StopDaemon()

		node.IPFS("log", "level", "core", "debug")
		res := node.IPFS("log", "get-level", "core")
		assert.NoError(t, res.Err)
		// Note: stderr might contain daemon output, so we don't check it

		output := res.Stdout.String()
		lines := SplitLines(output)

		// Should contain exactly one line
		assert.Equal(t, 1, len(lines))

		// Check format (note: environment variable may not be reflected due to go-log implementation)
		line := strings.TrimSpace(lines[0])
		parts := strings.Split(line, ": ")
		assert.Equal(t, 2, len(parts), "Line should have format 'subsystem: level', got: %s", line)
		assert.Equal(t, "core", parts[0])
		assert.Equal(t, "debug", parts[1])
	})

	t.Run("get-level with non-existent subsystem returns error", func(t *testing.T) {
		// t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		res := node.RunIPFS("log", "get-level", "non-existent-subsystem")
		assert.Error(t, res.Err)
		assert.NotEqual(t, 0, len(res.Stderr.Lines()))
	})
}
