package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForLogMessage polls a buffer for a log message, waiting up to timeout duration.
// Returns true if message found, false if timeout reached.
func waitForLogMessage(buffer *harness.Buffer, message string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(buffer.String(), message) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func TestAdd(t *testing.T) {
	t.Parallel()

	var (
		shortString                 = "hello world"
		shortStringCidV0            = "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD"              // cidv0 - dag-pb - sha2-256
		shortStringCidV1            = "bafkreifzjut3te2nhyekklss27nh3k72ysco7y32koao5eei66wof36n5e" // cidv1 - raw - sha2-256
		shortStringCidV1NoRawLeaves = "bafybeihykld7uyxzogax6vgyvag42y7464eywpf55gxi5qpoisibh3c5wa" // cidv1 - dag-pb - sha2-256
		shortStringCidV1Sha512      = "bafkrgqbqt3gerhas23vuzrapkdeqf4vu2dwxp3srdj6hvg6nhsug2tgyn6mj3u23yx7utftq3i2ckw2fwdh5qmhid5qf3t35yvkc5e5ottlw6"
	)

	t.Run("produced cid version: implicit default (CIDv0)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)
	})

	t.Run("produced cid version: follows user-set configuration Import.CidVersion=0", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(0)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)
	})

	t.Run("produced cid multihash: follows user-set configuration in Import.HashFunction", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.HashFunction = *config.NewOptionalString("sha2-512")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV1Sha512, cidStr)
	})

	t.Run("produced cid version: follows user-set configuration Import.CidVersion=1", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV1, cidStr)
	})

	t.Run("produced cid version: command flag overrides configuration in Import.CidVersion", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString, "--cid-version", "0")
		require.Equal(t, shortStringCidV0, cidStr)
	})

	t.Run("produced unixfs raw leaves: follows user-set configuration Import.UnixFSRawLeaves", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			// CIDv1 defaults to  raw-leaves=true
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
			// disable manually
			cfg.Import.UnixFSRawLeaves = config.False
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV1NoRawLeaves, cidStr)
	})

	t.Run("ipfs add --pin-name=foo", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		pinName := "test-pin-name"
		cidStr := node.IPFSAddStr(shortString, "--pin-name", pinName)
		require.Equal(t, shortStringCidV0, cidStr)

		pinList := node.IPFS("pin", "ls", "--names").Stdout.Trimmed()
		require.Contains(t, pinList, shortStringCidV0)
		require.Contains(t, pinList, pinName)
	})

	t.Run("ipfs add --pin=false --pin-name=foo returns an error", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// Use RunIPFS to allow for errors without assertion
		result := node.RunIPFS("add", "--pin=false", "--pin-name=foo")
		require.Error(t, result.Err, "Expected an error due to incompatible --pin and --pin-name")
		require.Contains(t, result.Stderr.String(), "pin-name option requires pin to be set")
	})

	t.Run("ipfs add --pin-name without value should fail", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// When --pin-name is passed without any value, it should fail
		result := node.RunIPFS("add", "--pin-name")
		require.Error(t, result.Err, "Expected an error when --pin-name has no value")
		require.Contains(t, result.Stderr.String(), "missing argument for option \"pin-name\"")
	})

	t.Run("produced unixfs max file links: command flag --max-file-links overrides configuration in Import.UnixFSFileMaxLinks", func(t *testing.T) {
		t.Parallel()

		//
		// UnixFSChunker=size-262144 (256KiB)
		// Import.UnixFSFileMaxLinks=174
		node := harness.NewT(t).NewNode().Init("--profile=unixfs-v0-2015") // unixfs-v0-2015 for determinism across all params
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.UnixFSChunker = *config.NewOptionalString("size-262144") // 256 KiB chunks
			cfg.Import.UnixFSFileMaxLinks = *config.NewOptionalInteger(174)     // max 174 per level
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Add 174MiB file:
		// 1024 * 256KiB should fit in single layer
		seed := shortString
		cidStr := node.IPFSAddDeterministic("262144KiB", seed, "--max-file-links", "1024")
		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)

		// Expect 1024 links due to cli parameter raising link limit from 174 to 1024
		require.Equal(t, 1024, len(root.Links))
		// expect same CID every time
		require.Equal(t, "QmbBftNHWmjSWKLC49dMVrfnY8pjrJYntiAXirFJ7oJrNk", cidStr)
	})

	// Profile-specific threshold tests are in cid_profiles_test.go (TestCIDProfiles).
	// Tests here cover general ipfs add behavior not tied to specific profiles.

	t.Run("ipfs add --hidden", func(t *testing.T) {
		t.Parallel()

		// Helper to create test directory with hidden file
		setupTestDir := func(t *testing.T, node *harness.Node) string {
			testDir, err := os.MkdirTemp(node.Dir, "hidden-test")
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(testDir, "visible.txt"), []byte("visible"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(testDir, ".hidden"), []byte("hidden"), 0o644))
			return testDir
		}

		t.Run("default excludes hidden files", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			testDir := setupTestDir(t, node)
			cidStr := node.IPFS("add", "-r", "-Q", testDir).Stdout.Trimmed()
			lsOutput := node.IPFS("ls", cidStr).Stdout.Trimmed()
			require.Contains(t, lsOutput, "visible.txt")
			require.NotContains(t, lsOutput, ".hidden")
		})

		t.Run("--hidden includes hidden files", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			testDir := setupTestDir(t, node)
			cidStr := node.IPFS("add", "-r", "-Q", "--hidden", testDir).Stdout.Trimmed()
			lsOutput := node.IPFS("ls", cidStr).Stdout.Trimmed()
			require.Contains(t, lsOutput, "visible.txt")
			require.Contains(t, lsOutput, ".hidden")
		})

		t.Run("-H includes hidden files", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			testDir := setupTestDir(t, node)
			cidStr := node.IPFS("add", "-r", "-Q", "-H", testDir).Stdout.Trimmed()
			lsOutput := node.IPFS("ls", cidStr).Stdout.Trimmed()
			require.Contains(t, lsOutput, "visible.txt")
			require.Contains(t, lsOutput, ".hidden")
		})
	})

	t.Run("ipfs add --empty-dirs", func(t *testing.T) {
		t.Parallel()

		// Helper to create test directory with empty subdirectory
		setupTestDir := func(t *testing.T, node *harness.Node) string {
			testDir, err := os.MkdirTemp(node.Dir, "empty-dirs-test")
			require.NoError(t, err)
			require.NoError(t, os.Mkdir(filepath.Join(testDir, "empty-subdir"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("content"), 0o644))
			return testDir
		}

		t.Run("default includes empty directories", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			testDir := setupTestDir(t, node)
			cidStr := node.IPFS("add", "-r", "-Q", testDir).Stdout.Trimmed()
			require.Contains(t, node.IPFS("ls", cidStr).Stdout.Trimmed(), "empty-subdir")
		})

		t.Run("--empty-dirs=true includes empty directories", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			testDir := setupTestDir(t, node)
			cidStr := node.IPFS("add", "-r", "-Q", "--empty-dirs=true", testDir).Stdout.Trimmed()
			require.Contains(t, node.IPFS("ls", cidStr).Stdout.Trimmed(), "empty-subdir")
		})

		t.Run("--empty-dirs=false excludes empty directories", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			testDir := setupTestDir(t, node)
			cidStr := node.IPFS("add", "-r", "-Q", "--empty-dirs=false", testDir).Stdout.Trimmed()
			lsOutput := node.IPFS("ls", cidStr).Stdout.Trimmed()
			require.NotContains(t, lsOutput, "empty-subdir")
			require.Contains(t, lsOutput, "file.txt")
		})
	})

	t.Run("ipfs add symlink handling", func(t *testing.T) {
		t.Parallel()

		// Helper to create test directory structure:
		// testDir/
		//   target.txt           (file with "target content")
		//   link.txt -> target.txt (symlink at top level)
		//   subdir/
		//     subsubdir/
		//       nested-target.txt (file with "nested content")
		//       nested-link.txt -> nested-target.txt (symlink in sub-sub directory)
		setupTestDir := func(t *testing.T, node *harness.Node) string {
			testDir, err := os.MkdirTemp(node.Dir, "deref-symlinks-test")
			require.NoError(t, err)

			// Top-level file and symlink
			targetFile := filepath.Join(testDir, "target.txt")
			require.NoError(t, os.WriteFile(targetFile, []byte("target content"), 0o644))
			require.NoError(t, os.Symlink("target.txt", filepath.Join(testDir, "link.txt")))

			// Nested file and symlink in sub-sub directory
			subsubdir := filepath.Join(testDir, "subdir", "subsubdir")
			require.NoError(t, os.MkdirAll(subsubdir, 0o755))
			nestedTarget := filepath.Join(subsubdir, "nested-target.txt")
			require.NoError(t, os.WriteFile(nestedTarget, []byte("nested content"), 0o644))
			require.NoError(t, os.Symlink("nested-target.txt", filepath.Join(subsubdir, "nested-link.txt")))

			return testDir
		}

		t.Run("default preserves symlinks", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			testDir := setupTestDir(t, node)

			// Add directory with symlink (default: preserve)
			dirCID := node.IPFS("add", "-r", "-Q", testDir).Stdout.Trimmed()

			// Get and verify symlinks are preserved
			outDir, err := os.MkdirTemp(node.Dir, "symlink-get-out")
			require.NoError(t, err)
			node.IPFS("get", "-o", outDir, dirCID)

			// Check top-level symlink is preserved
			linkPath := filepath.Join(outDir, "link.txt")
			fi, err := os.Lstat(linkPath)
			require.NoError(t, err)
			require.True(t, fi.Mode()&os.ModeSymlink != 0, "link.txt should be a symlink")
			target, err := os.Readlink(linkPath)
			require.NoError(t, err)
			require.Equal(t, "target.txt", target)

			// Check nested symlink is preserved
			nestedLinkPath := filepath.Join(outDir, "subdir", "subsubdir", "nested-link.txt")
			fi, err = os.Lstat(nestedLinkPath)
			require.NoError(t, err)
			require.True(t, fi.Mode()&os.ModeSymlink != 0, "nested-link.txt should be a symlink")
		})

		// --dereference-args is deprecated but still works for backwards compatibility.
		// It only resolves symlinks passed as CLI arguments, NOT symlinks found
		// during directory traversal. Use --dereference-symlinks instead.
		t.Run("--dereference-args resolves CLI args only", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			testDir := setupTestDir(t, node)
			symlinkPath := filepath.Join(testDir, "link.txt")
			targetPath := filepath.Join(testDir, "target.txt")

			symlinkCID := node.IPFS("add", "-Q", "--dereference-args", symlinkPath).Stdout.Trimmed()
			targetCID := node.IPFS("add", "-Q", targetPath).Stdout.Trimmed()

			// CIDs should match because --dereference-args resolves the symlink
			require.Equal(t, targetCID, symlinkCID,
				"--dereference-args should resolve CLI arg symlink to target content")

			// Now add the directory recursively with --dereference-args
			// Nested symlinks should NOT be resolved (only CLI args are resolved)
			dirCID := node.IPFS("add", "-r", "-Q", "--dereference-args", testDir).Stdout.Trimmed()

			outDir, err := os.MkdirTemp(node.Dir, "deref-args-out")
			require.NoError(t, err)
			node.IPFS("get", "-o", outDir, dirCID)

			// Nested symlink should still be a symlink (not dereferenced)
			nestedLinkPath := filepath.Join(outDir, "subdir", "subsubdir", "nested-link.txt")
			fi, err := os.Lstat(nestedLinkPath)
			require.NoError(t, err)
			require.True(t, fi.Mode()&os.ModeSymlink != 0,
				"--dereference-args should NOT resolve nested symlinks, only CLI args")
		})

		// --dereference-symlinks resolves ALL symlinks: both CLI arguments AND
		// symlinks found during directory traversal. This is a superset of
		// the deprecated --dereference-args behavior.
		t.Run("--dereference-symlinks resolves all symlinks", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			defer node.StopDaemon()

			testDir := setupTestDir(t, node)
			symlinkPath := filepath.Join(testDir, "link.txt")
			targetPath := filepath.Join(testDir, "target.txt")

			symlinkCID := node.IPFS("add", "-Q", "--dereference-symlinks", symlinkPath).Stdout.Trimmed()
			targetCID := node.IPFS("add", "-Q", targetPath).Stdout.Trimmed()

			require.Equal(t, targetCID, symlinkCID,
				"--dereference-symlinks should resolve CLI arg symlink (like --dereference-args)")

			// Test 2: Nested symlinks in sub-sub directory are ALSO resolved
			dirCID := node.IPFS("add", "-r", "-Q", "--dereference-symlinks", testDir).Stdout.Trimmed()

			outDir, err := os.MkdirTemp(node.Dir, "deref-symlinks-out")
			require.NoError(t, err)
			node.IPFS("get", "-o", outDir, dirCID)

			// Top-level symlink should be dereferenced to regular file
			linkPath := filepath.Join(outDir, "link.txt")
			fi, err := os.Lstat(linkPath)
			require.NoError(t, err)
			require.False(t, fi.Mode()&os.ModeSymlink != 0,
				"link.txt should be dereferenced to regular file")
			content, err := os.ReadFile(linkPath)
			require.NoError(t, err)
			require.Equal(t, "target content", string(content))

			// Nested symlink in sub-sub directory should ALSO be dereferenced
			nestedLinkPath := filepath.Join(outDir, "subdir", "subsubdir", "nested-link.txt")
			fi, err = os.Lstat(nestedLinkPath)
			require.NoError(t, err)
			require.False(t, fi.Mode()&os.ModeSymlink != 0,
				"nested-link.txt should be dereferenced (--dereference-symlinks resolves ALL symlinks)")
			nestedContent, err := os.ReadFile(nestedLinkPath)
			require.NoError(t, err)
			require.Equal(t, "nested content", string(nestedContent))
		})
	})
}

func TestAddFastProvide(t *testing.T) {
	t.Parallel()

	const (
		shortString      = "hello world"
		shortStringCidV0 = "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD" // cidv0 - dag-pb - sha2-256
	)

	t.Run("fast-provide-root disabled via config: verify skipped in logs", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideRoot = config.False
		})

		// Start daemon with debug logging
		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)

		// Verify fast-provide-root was disabled
		daemonLog := node.Daemon.Stderr.String()
		require.Contains(t, daemonLog, "fast-provide-root: skipped")
	})

	t.Run("fast-provide-root enabled with wait=false: verify async provide", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		// Use default config (FastProvideRoot=true, FastProvideWait=false)

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)

		daemonLog := node.Daemon.Stderr
		// Should see async mode started
		require.Contains(t, daemonLog.String(), "fast-provide-root: enabled")
		require.Contains(t, daemonLog.String(), "fast-provide-root: providing asynchronously")

		// Wait for async completion or failure (up to 11 seconds - slightly more than fastProvideTimeout)
		// In test environment with no DHT peers, this will fail with "failed to find any peer in table"
		completedOrFailed := waitForLogMessage(daemonLog, "async provide completed", 11*time.Second) ||
			waitForLogMessage(daemonLog, "async provide failed", 11*time.Second)
		require.True(t, completedOrFailed, "async provide should complete or fail within timeout")
	})

	t.Run("fast-provide-root enabled with wait=true: verify sync provide", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideWait = config.True
		})

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		// Use Runner.Run with stdin to allow for expected errors
		res := node.Runner.Run(harness.RunRequest{
			Path: node.IPFSBin,
			Args: []string{"add", "-q"},
			CmdOpts: []harness.CmdOpt{
				harness.RunWithStdin(strings.NewReader(shortString)),
			},
		})

		// In sync mode (wait=true), provide errors propagate and fail the command.
		// Test environment uses 'test' profile with no bootstrappers, and CI has
		// insufficient peers for proper DHT puts, so we expect this to fail with
		// "failed to find any peer in table" error from the DHT.
		require.Equal(t, 1, res.ExitCode())
		require.Contains(t, res.Stderr.String(), "Error: fast-provide: failed to find any peer in table")

		daemonLog := node.Daemon.Stderr.String()
		// Should see sync mode started
		require.Contains(t, daemonLog, "fast-provide-root: enabled")
		require.Contains(t, daemonLog, "fast-provide-root: providing synchronously")
		require.Contains(t, daemonLog, "sync provide failed") // Verify the failure was logged
	})

	t.Run("fast-provide-wait ignored when root disabled", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideRoot = config.False
			cfg.Import.FastProvideWait = config.True
		})

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)

		daemonLog := node.Daemon.Stderr.String()
		require.Contains(t, daemonLog, "fast-provide-root: skipped")
		require.Contains(t, daemonLog, "wait-flag-ignored")
	})

	t.Run("CLI flag overrides config: flag=true overrides config=false", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideRoot = config.False
		})

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString, "--fast-provide-root=true")
		require.Equal(t, shortStringCidV0, cidStr)

		daemonLog := node.Daemon.Stderr
		// Flag should enable it despite config saying false
		require.Contains(t, daemonLog.String(), "fast-provide-root: enabled")
		require.Contains(t, daemonLog.String(), "fast-provide-root: providing asynchronously")
	})

	t.Run("CLI flag overrides config: flag=false overrides config=true", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.FastProvideRoot = config.True
		})

		node.StartDaemonWithReq(harness.RunRequest{
			CmdOpts: []harness.CmdOpt{
				harness.RunWithEnv(map[string]string{
					"GOLOG_LOG_LEVEL": "error,core/commands=debug,core/commands/cmdenv=debug",
				}),
			},
		}, "")
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString, "--fast-provide-root=false")
		require.Equal(t, shortStringCidV0, cidStr)

		daemonLog := node.Daemon.Stderr.String()
		// Flag should disable it despite config saying true
		require.Contains(t, daemonLog, "fast-provide-root: skipped")
	})
}

// createDirectoryForHAMTLinksEstimation creates a directory with the specified number
// of files for testing links-based size estimation (size = sum of nameLen + cidLen).
// Used by legacy profiles (unixfs-v0-2015).
//
// The lastNameLen parameter allows the last file to have a different name length,
// enabling exact +1 byte threshold tests.
func createDirectoryForHAMTLinksEstimation(dirPath string, numFiles, nameLen, lastNameLen int, seed string) error {
	return createDeterministicFiles(dirPath, numFiles, nameLen, lastNameLen, seed)
}

// createDirectoryForHAMTBlockEstimation creates a directory with the specified number
// of files for testing block-based size estimation (LinkSerializedSize with protobuf overhead).
// Used by modern profiles (unixfs-v1-2025).
//
// The lastNameLen parameter allows the last file to have a different name length,
// enabling exact +1 byte threshold tests.
func createDirectoryForHAMTBlockEstimation(dirPath string, numFiles, nameLen, lastNameLen int, seed string) error {
	return createDeterministicFiles(dirPath, numFiles, nameLen, lastNameLen, seed)
}

// createDeterministicFiles creates numFiles files with deterministic names.
// Files 0 to numFiles-2 have nameLen characters, and the last file has lastNameLen characters.
// Each file contains "x" (1 byte) for non-zero tsize in directory links.
func createDeterministicFiles(dirPath string, numFiles, nameLen, lastNameLen int, seed string) error {
	alphabetLen := len(testutils.AlphabetEasy)

	// Deterministic pseudo-random bytes for static filenames
	drand, err := testutils.DeterministicRandomReader("1MiB", seed)
	if err != nil {
		return err
	}

	for i := range numFiles {
		// Use lastNameLen for the final file
		currentNameLen := nameLen
		if i == numFiles-1 {
			currentNameLen = lastNameLen
		}

		buf := make([]byte, currentNameLen)
		_, err := io.ReadFull(drand, buf)
		if err != nil {
			return err
		}

		// Convert deterministic pseudo-random bytes to ASCII
		var sb strings.Builder
		for _, b := range buf {
			char := testutils.AlphabetEasy[int(b)%alphabetLen]
			sb.WriteRune(char)
		}
		filename := sb.String()[:currentNameLen]
		filePath := filepath.Join(dirPath, filename)

		// Create file with 1-byte content for non-zero tsize
		if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
			return err
		}
	}
	return nil
}
