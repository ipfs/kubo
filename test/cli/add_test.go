package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
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

	const (
		cidV0Length = 34 // cidv0 sha2-256
		cidV1Length = 36 // cidv1 sha2-256
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
		node := harness.NewT(t).NewNode().Init("--profile=legacy-cid-v0") // legacy-cid-v0 for determinism across all params
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

	t.Run("ipfs init --profile=legacy-cid-v0 sets config that produces legacy CIDv0", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init("--profile=legacy-cid-v0")
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)
	})

	t.Run("ipfs init --profile=legacy-cid-v0 applies UnixFSChunker=size-262144 and UnixFSFileMaxLinks", func(t *testing.T) {
		t.Parallel()
		seed := "v0-seed"
		profile := "--profile=legacy-cid-v0"

		t.Run("under UnixFSFileMaxLinks=174", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()
			// Add 44544KiB file:
			// 174 * 256KiB should fit in single DAG layer
			cidStr := node.IPFSAddDeterministic("44544KiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 174, len(root.Links))
			// expect same CID every time
			require.Equal(t, "QmUbBALi174SnogsUzLpYbD4xPiBSFANF4iztWCsHbMKh2", cidStr)
		})

		t.Run("above UnixFSFileMaxLinks=174", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()
			// add 256KiB (one more block), it should force rebalancing DAG and moving most to second layer
			cidStr := node.IPFSAddDeterministic("44800KiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 2, len(root.Links))
			// expect same CID every time
			require.Equal(t, "QmepeWtdmS1hHXx1oZXsPUv6bMrfRRKfZcoPPU4eEfjnbf", cidStr)
		})
	})

	t.Run("ipfs init --profile=legacy-cid-v0 applies UnixFSHAMTDirectoryMaxFanout=256 and UnixFSHAMTDirectorySizeThreshold=256KiB", func(t *testing.T) {
		t.Parallel()
		seed := "hamt-legacy-cid-v0"
		profile := "--profile=legacy-cid-v0"

		t.Run("under UnixFSHAMTDirectorySizeThreshold=256KiB", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()

			randDir, err := os.MkdirTemp(node.Dir, seed)
			require.NoError(t, err)

			// Create directory with a lot of files that have filenames which together take close to UnixFSHAMTDirectorySizeThreshold in total
			err = createDirectoryForHAMT(randDir, cidV0Length, "255KiB", seed)
			require.NoError(t, err)
			cidStr := node.IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()

			// Confirm the number of links is more than UnixFSHAMTDirectorySizeThreshold (indicating regular "basic" directory"
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 903, len(root.Links))
		})

		t.Run("above UnixFSHAMTDirectorySizeThreshold=256KiB", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()

			randDir, err := os.MkdirTemp(node.Dir, seed)
			require.NoError(t, err)

			// Create directory with a lot of files that have filenames which together take close to UnixFSHAMTDirectorySizeThreshold in total
			err = createDirectoryForHAMT(randDir, cidV0Length, "257KiB", seed)
			require.NoError(t, err)
			cidStr := node.IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()

			// Confirm this time, the number of links is less than UnixFSHAMTDirectorySizeThreshold
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 252, len(root.Links))
		})
	})

	t.Run("ipfs init --profile=test-cid-v1 produces CIDv1 with raw leaves", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init("--profile=test-cid-v1")
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV1, cidStr) // raw leaf
	})

	t.Run("ipfs init --profile=test-cid-v1 applies UnixFSChunker=size-1048576", func(t *testing.T) {
		t.Parallel()
		seed := "v1-seed"
		profile := "--profile=test-cid-v1"

		t.Run("under UnixFSFileMaxLinks=174", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()
			// Add 174MiB file:
			// 174 * 1MiB should fit in single layer
			cidStr := node.IPFSAddDeterministic("174MiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 174, len(root.Links))
			// expect same CID every time
			require.Equal(t, "bafybeigwduxcf2aawppv3isnfeshnimkyplvw3hthxjhr2bdeje4tdaicu", cidStr)
		})

		t.Run("above UnixFSFileMaxLinks=174", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()
			// add +1MiB (one more block), it should force rebalancing DAG and moving most to second layer
			cidStr := node.IPFSAddDeterministic("175MiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 2, len(root.Links))
			// expect same CID every time
			require.Equal(t, "bafybeidhd7lo2n2v7lta5yamob3xwhbxcczmmtmhquwhjesi35jntf7mpu", cidStr)
		})
	})

	t.Run("ipfs init --profile=test-cid-v1 applies UnixFSHAMTDirectoryMaxFanout=256 and UnixFSHAMTDirectorySizeThreshold=256KiB", func(t *testing.T) {
		t.Parallel()
		seed := "hamt-cid-v1"
		profile := "--profile=test-cid-v1"

		t.Run("under UnixFSHAMTDirectorySizeThreshold=256KiB", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()

			randDir, err := os.MkdirTemp(node.Dir, seed)
			require.NoError(t, err)

			// Create directory with a lot of files that have filenames which together take close to UnixFSHAMTDirectorySizeThreshold in total
			err = createDirectoryForHAMT(randDir, cidV1Length, "255KiB", seed)
			require.NoError(t, err)
			cidStr := node.IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()

			// Confirm the number of links is more than UnixFSHAMTDirectoryMaxFanout (indicating regular "basic" directory"
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 897, len(root.Links))
		})

		t.Run("above UnixFSHAMTDirectorySizeThreshold=256KiB", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()

			randDir, err := os.MkdirTemp(node.Dir, seed)
			require.NoError(t, err)

			// Create directory with a lot of files that have filenames which together take close to UnixFSHAMTDirectorySizeThreshold in total
			err = createDirectoryForHAMT(randDir, cidV1Length, "257KiB", seed)
			require.NoError(t, err)
			cidStr := node.IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()

			// Confirm this time, the number of links is less than UnixFSHAMTDirectoryMaxFanout
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 252, len(root.Links))
		})
	})

	t.Run("ipfs init --profile=test-cid-v1-wide applies UnixFSChunker=size-1048576 and UnixFSFileMaxLinks=1024", func(t *testing.T) {
		t.Parallel()
		seed := "v1-seed-1024"
		profile := "--profile=test-cid-v1-wide"

		t.Run("under UnixFSFileMaxLinks=1024", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()
			// Add 174MiB file:
			// 1024 * 1MiB should fit in single layer
			cidStr := node.IPFSAddDeterministic("1024MiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 1024, len(root.Links))
			// expect same CID every time
			require.Equal(t, "bafybeiej5w63ir64oxgkr5htqmlerh5k2rqflurn2howimexrlkae64xru", cidStr)
		})

		t.Run("above UnixFSFileMaxLinks=1024", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()
			// add +1MiB (one more block), it should force rebalancing DAG and moving most to second layer
			cidStr := node.IPFSAddDeterministic("1025MiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 2, len(root.Links))
			// expect same CID every time
			require.Equal(t, "bafybeieilp2qx24pe76hxrxe6bpef5meuxto3kj5dd6mhb5kplfeglskdm", cidStr)
		})
	})

	t.Run("ipfs init --profile=test-cid-v1-wide applies UnixFSHAMTDirectoryMaxFanout=256 and UnixFSHAMTDirectorySizeThreshold=1MiB", func(t *testing.T) {
		t.Parallel()
		seed := "hamt-cid-v1"
		profile := "--profile=test-cid-v1-wide"

		t.Run("under UnixFSHAMTDirectorySizeThreshold=1MiB", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()

			randDir, err := os.MkdirTemp(node.Dir, seed)
			require.NoError(t, err)

			// Create directory with a lot of files that have filenames which together take close to UnixFSHAMTDirectorySizeThreshold in total
			err = createDirectoryForHAMT(randDir, cidV1Length, "1023KiB", seed)
			require.NoError(t, err)
			cidStr := node.IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()

			// Confirm the number of links is more than UnixFSHAMTDirectoryMaxFanout (indicating regular "basic" directory"
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 3599, len(root.Links))
		})

		t.Run("above UnixFSHAMTDirectorySizeThreshold=1MiB", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init(profile)
			node.StartDaemon()
			defer node.StopDaemon()

			randDir, err := os.MkdirTemp(node.Dir, seed)
			require.NoError(t, err)

			// Create directory with a lot of files that have filenames which together take close to UnixFSHAMTDirectorySizeThreshold in total
			err = createDirectoryForHAMT(randDir, cidV1Length, "1025KiB", seed)
			require.NoError(t, err)
			cidStr := node.IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()

			// Confirm this time, the number of links is less than UnixFSHAMTDirectoryMaxFanout
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 992, len(root.Links))
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

// createDirectoryForHAMT aims to create enough files with long names for the directory block to be close to the UnixFSHAMTDirectorySizeThreshold.
// The calculation is based on boxo's HAMTShardingSize and sizeBelowThreshold which calculates ballpark size of the block
// by adding length of link names and the binary cid length.
// See https://github.com/ipfs/boxo/blob/6c5a07602aed248acc86598f30ab61923a54a83e/ipld/unixfs/io/directory.go#L491
func createDirectoryForHAMT(dirPath string, cidLength int, unixfsNodeSizeTarget, seed string) error {
	hamtThreshold, err := humanize.ParseBytes(unixfsNodeSizeTarget)
	if err != nil {
		return err
	}

	// Calculate how many files with long filenames are needed to hit UnixFSHAMTDirectorySizeThreshold
	nameLen := 255 // max that works across windows/macos/linux
	alphabetLen := len(testutils.AlphabetEasy)
	numFiles := int(hamtThreshold) / (nameLen + cidLength)

	// Deterministic pseudo-random bytes for static CID
	drand, err := testutils.DeterministicRandomReader(unixfsNodeSizeTarget, seed)
	if err != nil {
		return err
	}

	// Create necessary files in a single, flat directory
	for i := 0; i < numFiles; i++ {
		buf := make([]byte, nameLen)
		_, err := io.ReadFull(drand, buf)
		if err != nil {
			return err
		}

		// Convert deterministic pseudo-random bytes to ASCII
		var sb strings.Builder

		for _, b := range buf {
			// Map byte to printable ASCII range (33-126)
			char := testutils.AlphabetEasy[int(b)%alphabetLen]
			sb.WriteRune(char)
		}
		filename := sb.String()[:nameLen]
		filePath := filepath.Join(dirPath, filename)

		// Create empty file
		f, err := os.Create(filePath)
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}
