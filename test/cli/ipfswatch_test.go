//go:build !plan9

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

func TestIPFSWatch(t *testing.T) {
	t.Parallel()

	// Build ipfswatch binary once before running parallel subtests.
	// This avoids race conditions and duplicate builds.
	h := harness.NewT(t)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(h.IPFSBin)))
	ipfswatchBin := filepath.Join(repoRoot, "cmd", "ipfswatch", "ipfswatch")

	if _, err := os.Stat(ipfswatchBin); os.IsNotExist(err) {
		// -C changes to repo root so go.mod is found
		cmd := exec.Command("go", "build", "-C", repoRoot, "-o", ipfswatchBin, "./cmd/ipfswatch")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "failed to build ipfswatch: %s", string(out))
	}

	t.Run("ipfswatch adds watched files to IPFS", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		// Create a temp directory to watch
		watchDir := filepath.Join(h.Dir, "watch")
		err := os.MkdirAll(watchDir, 0o755)
		require.NoError(t, err)

		// Start ipfswatch in background
		result := node.Runner.Run(harness.RunRequest{
			Path:    ipfswatchBin,
			Args:    []string{"--repo", node.Dir, "--path", watchDir},
			RunFunc: harness.RunFuncStart,
		})
		require.NoError(t, result.Err, "ipfswatch should start without error")
		defer func() {
			if result.Cmd.Process != nil {
				_ = result.Cmd.Process.Kill()
				_, _ = result.Cmd.Process.Wait()
			}
		}()

		// Wait for ipfswatch to initialize
		time.Sleep(2 * time.Second)

		// Check for startup errors
		stderrStr := result.Stderr.String()
		require.NotContains(t, stderrStr, "unknown datastore type", "ipfswatch should recognize datastore plugins")

		// Create a test file with unique content based on timestamp
		testContent := fmt.Sprintf("ipfswatch test content generated at %s", time.Now().Format(time.RFC3339Nano))
		testFile := filepath.Join(watchDir, "test.txt")
		err = os.WriteFile(testFile, []byte(testContent), 0o644)
		require.NoError(t, err)

		// Wait for ipfswatch to process the file and extract CID from log
		// Log format: "added %s... key: %s"
		cidPattern := regexp.MustCompile(`added .*/test\.txt\.\.\. key: (\S+)`)
		var cid string
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			stderrStr = result.Stderr.String()
			if matches := cidPattern.FindStringSubmatch(stderrStr); len(matches) > 1 {
				cid = matches[1]
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		require.NotEmpty(t, cid, "ipfswatch should have added test.txt and logged the CID, got stderr: %s", stderrStr)

		// Kill ipfswatch to release the repo lock
		if result.Cmd.Process != nil {
			if err = result.Cmd.Process.Signal(os.Interrupt); err != nil {
				_ = result.Cmd.Process.Kill()
			}
			_, _ = result.Cmd.Process.Wait()
		}

		// Verify the content matches by reading it back via ipfs cat
		catRes := node.RunIPFS("cat", "--offline", cid)
		require.Equal(t, 0, catRes.Cmd.ProcessState.ExitCode(),
			"ipfs cat should succeed, cid=%s, stderr: %s", cid, catRes.Stderr.String())
		require.Equal(t, testContent, catRes.Stdout.String(),
			"content read from IPFS should match what was written")
	})

	t.Run("ipfswatch loads datastore plugins for pebbleds", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()

		// Configure pebbleds as the datastore
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Datastore.Spec = map[string]interface{}{
				"type": "mount",
				"mounts": []interface{}{
					map[string]interface{}{
						"mountpoint": "/blocks",
						"path":       "blocks",
						"prefix":     "flatfs.datastore",
						"shardFunc":  "/repo/flatfs/shard/v1/next-to-last/2",
						"sync":       true,
						"type":       "flatfs",
					},
					map[string]interface{}{
						"mountpoint": "/",
						"path":       "datastore",
						"prefix":     "pebble.datastore",
						"type":       "pebbleds",
					},
				},
			}
		})

		// Re-initialize datastore directory for pebbleds
		// (the repo was initialized with levelds, need to remove it)
		dsPath := filepath.Join(node.Dir, "datastore")
		err := os.RemoveAll(dsPath)
		require.NoError(t, err)
		err = os.MkdirAll(dsPath, 0o755)
		require.NoError(t, err)

		// Create a temp directory to watch
		watchDir := filepath.Join(h.Dir, "watch")
		err = os.MkdirAll(watchDir, 0o755)
		require.NoError(t, err)

		// Start ipfswatch in background
		result := node.Runner.Run(harness.RunRequest{
			Path:    ipfswatchBin,
			Args:    []string{"--repo", node.Dir, "--path", watchDir},
			RunFunc: harness.RunFuncStart,
		})
		require.NoError(t, result.Err, "ipfswatch should start without error")
		defer func() {
			if result.Cmd.Process != nil {
				_ = result.Cmd.Process.Kill()
				_, _ = result.Cmd.Process.Wait()
			}
		}()

		// Wait for ipfswatch to initialize and check for errors
		time.Sleep(3 * time.Second)

		stderrStr := result.Stderr.String()
		require.NotContains(t, stderrStr, "unknown datastore type", "ipfswatch should recognize pebbleds datastore plugin")
	})
}
