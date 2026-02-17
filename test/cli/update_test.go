package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdate exercises the built-in "ipfs update" command tree against
// the real GitHub Releases API. Network access is required.
//
// The node is created without Init or daemon, so install/revert error
// paths that don't depend on a running daemon can be tested.
func TestUpdate(t *testing.T) {
	t.Parallel()
	h := harness.NewT(t)
	node := h.NewNode()

	t.Run("help text describes the command", func(t *testing.T) {
		t.Parallel()
		res := node.IPFS("update", "--help")
		assert.Contains(t, res.Stdout.String(), "Update Kubo to a different version")
	})

	// check and versions are read-only GitHub API queries. They must work
	// regardless of daemon state, since users need to check for updates
	// before deciding whether to stop the daemon and install.
	t.Run("check", func(t *testing.T) {
		t.Parallel()

		t.Run("text output reports update availability", func(t *testing.T) {
			t.Parallel()
			res := node.IPFS("update", "check")
			out := res.Stdout.String()
			assert.True(t,
				strings.Contains(out, "Update available") || strings.Contains(out, "Already up to date"),
				"expected update status message, got: %s", out)
		})

		t.Run("json output includes version fields", func(t *testing.T) {
			t.Parallel()
			res := node.IPFS("update", "check", "--enc=json")
			var result struct {
				CurrentVersion  string
				LatestVersion   string
				UpdateAvailable bool
			}
			err := json.Unmarshal(res.Stdout.Bytes(), &result)
			require.NoError(t, err, "invalid JSON: %s", res.Stdout.String())
			assert.NotEmpty(t, result.CurrentVersion, "must report current version")
			assert.NotEmpty(t, result.LatestVersion, "must report latest version")
		})
	})

	t.Run("versions", func(t *testing.T) {
		t.Parallel()

		t.Run("lists available versions", func(t *testing.T) {
			t.Parallel()
			res := node.IPFS("update", "versions")
			lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
			assert.Greater(t, len(lines), 0, "should list at least one version")
		})

		t.Run("respects --count flag", func(t *testing.T) {
			t.Parallel()
			res := node.IPFS("update", "versions", "--count=5")
			lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
			assert.LessOrEqual(t, len(lines), 5)
		})

		t.Run("json output includes current version and list", func(t *testing.T) {
			t.Parallel()
			res := node.IPFS("update", "versions", "--count=3", "--enc=json")
			var result struct {
				Current  string
				Versions []string
			}
			err := json.Unmarshal(res.Stdout.Bytes(), &result)
			require.NoError(t, err, "invalid JSON: %s", res.Stdout.String())
			assert.NotEmpty(t, result.Current, "must report current version")
			assert.NotEmpty(t, result.Versions, "must list at least one version")
		})

		t.Run("--pre includes prerelease versions", func(t *testing.T) {
			t.Parallel()
			res := node.IPFS("update", "versions", "--count=5", "--pre")
			lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
			assert.Greater(t, len(lines), 0, "should list at least one version")
		})
	})

	// install and revert mutate the binary on disk, so they have stricter
	// preconditions. These tests verify the error paths.
	t.Run("install rejects same version", func(t *testing.T) {
		t.Parallel()
		vRes := node.IPFS("version", "-n")
		current := strings.TrimSpace(vRes.Stdout.String())

		res := node.RunIPFS("update", "install", current)
		assert.Error(t, res.Err)
		assert.Contains(t, res.Stderr.String(), "already running version",
			"should refuse to re-install the current version")
	})

	t.Run("revert fails when no backup exists", func(t *testing.T) {
		t.Parallel()
		res := node.RunIPFS("update", "revert")
		assert.Error(t, res.Err)
		assert.Contains(t, res.Stderr.String(), "no stashed binaries",
			"should explain there is no previous version to restore")
	})
}

// TestUpdateWhileDaemonRuns verifies that read-only update subcommands
// (check, versions) work while the IPFS daemon holds the repo lock.
// These commands only query the GitHub API and never touch the repo,
// so they must succeed regardless of daemon state.
func TestUpdateWhileDaemonRuns(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init().StartDaemon()
	defer node.StopDaemon()

	t.Run("check succeeds with daemon running", func(t *testing.T) {
		t.Parallel()
		res := node.IPFS("update", "check")
		out := res.Stdout.String()
		assert.True(t,
			strings.Contains(out, "Update available") || strings.Contains(out, "Already up to date"),
			"check must work while daemon runs, got: %s", out)
	})

	t.Run("versions succeeds with daemon running", func(t *testing.T) {
		t.Parallel()
		res := node.IPFS("update", "versions", "--count=3")
		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assert.Greater(t, len(lines), 0,
			"versions must work while daemon runs")
	})
}

// TestUpdateInstall exercises the full install flow end-to-end:
// API query, archive download, SHA-512 verification, tar.gz extraction,
// binary stash (backup), and atomic replace.
//
// A local mock HTTP server replaces GitHub so the test is fast, offline,
// and deterministic. The built ipfs binary is copied to a temp directory
// so the install replaces the copy, not the real build artifact.
//
// The env var KUBO_UPDATE_GITHUB_URL redirects the binary's GitHub API
// calls to the mock server. IPFS_VERSION_FAKE makes the binary report
// an older version so the "upgrade" to v0.99.0 is accepted.
func TestUpdateInstall(t *testing.T) {
	t.Parallel()

	// Build a fake binary to put inside the archive. After install, the
	// file at tmpBinPath should contain exactly these bytes.
	fakeBinary := []byte("#!/bin/sh\necho fake-ipfs-v0.99.0\n")

	// Archive entry path: extractBinaryFromArchive looks for "kubo/<exename>".
	binName := "ipfs"
	if runtime.GOOS == "windows" {
		binName = "ipfs.exe"
	}
	var archive []byte
	if runtime.GOOS == "windows" {
		archive = buildTestZip(t, "kubo/"+binName, fakeBinary)
	} else {
		archive = buildTestTarGz(t, "kubo/"+binName, fakeBinary)
	}

	// Compute SHA-512 of the archive for the .sha512 sidecar file.
	sum := sha512.Sum512(archive)

	// Asset name must match what findReleaseAsset expects for the
	// current OS/arch (e.g., kubo_v0.99.0_linux-amd64.tar.gz).
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	assetName := fmt.Sprintf("kubo_v0.99.0_%s-%s.%s", runtime.GOOS, runtime.GOARCH, ext)
	checksumBody := fmt.Sprintf("%x  %s\n", sum[:], assetName)

	// Mock server: serves GitHub Releases API, archive, and .sha512 sidecar.
	// srvURL is captured after the server starts, so the handler can build
	// browser_download_url values pointing back to itself.
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		// githubReleaseByTag: GET /tags/v0.99.0
		case r.URL.Path == "/tags/v0.99.0":
			rel := map[string]any{
				"tag_name":   "v0.99.0",
				"prerelease": false,
				"assets": []map[string]any{{
					"name":                 assetName,
					"browser_download_url": srvURL + "/download/" + assetName,
				}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rel)

		// downloadAsset: GET /download/<asset>.tar.gz
		case r.URL.Path == "/download/"+assetName:
			_, _ = w.Write(archive)

		// downloadAndVerifySHA512: GET /download/<asset>.tar.gz.sha512
		case r.URL.Path == "/download/"+assetName+".sha512":
			_, _ = w.Write([]byte(checksumBody))

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	// Copy the real built binary to a temp directory. The install command
	// uses os.Executable() to find the binary to replace, so the subprocess
	// will replace this copy instead of the real build artifact.
	tmpBinDir := t.TempDir()
	tmpBinPath := filepath.Join(tmpBinDir, binName)
	copyBuiltBinary(t, tmpBinPath)

	// Create a harness that uses the temp binary copy.
	h := harness.NewT(t, func(h *harness.Harness) {
		h.IPFSBin = tmpBinPath
	})
	node := h.NewNode()

	// Make the binary think it's running v0.30.0 so the "upgrade" to v0.99.0
	// is accepted. Point API calls at the mock server.
	node.Runner.Env["IPFS_VERSION_FAKE"] = "0.30.0"
	node.Runner.Env["KUBO_UPDATE_GITHUB_URL"] = srvURL

	// Run: ipfs update install v0.99.0
	res := node.RunIPFS("update", "install", "v0.99.0")
	require.NoError(t, res.Err, "install failed; stderr:\n%s", res.Stderr.String())

	// Verify progress messages on stderr.
	stderr := res.Stderr.String()
	assert.Contains(t, stderr, "Downloading Kubo 0.99.0",
		"should show download progress")
	assert.Contains(t, stderr, "Checksum verified (SHA-512)",
		"should confirm checksum passed")
	assert.Contains(t, stderr, "Backed up current binary to",
		"should report where the old binary was stashed")
	assert.Contains(t, stderr, "Successfully updated Kubo 0.30.0 -> 0.99.0",
		"should confirm the version change")

	// Verify the stash: the original binary should be saved to
	// $IPFS_PATH/old-bin/ipfs-0.30.0.
	stashPath := filepath.Join(node.Dir, "old-bin", "ipfs-0.30.0")
	_, err := os.Stat(stashPath)
	require.NoError(t, err, "stash file should exist at %s", stashPath)

	// Verify the binary was replaced with the fake binary from the archive.
	got, err := os.ReadFile(tmpBinPath)
	require.NoError(t, err)
	assert.Equal(t, fakeBinary, got,
		"binary at %s should contain the extracted archive content", tmpBinPath)
}

// TestUpdateRevert exercises the full revert flow end-to-end: reading
// a stashed binary from $IPFS_PATH/old-bin/, atomically replacing the
// current binary, and cleaning up the stash file.
//
// The stash is created manually (rather than via install) so this test
// is self-contained and does not depend on network access or a mock server.
//
// How it works: the subprocess runs from tmpBinPath, so os.Executable()
// inside the subprocess returns tmpBinPath. The revert command reads the
// stash and atomically replaces the file at tmpBinPath with stash content.
func TestUpdateRevert(t *testing.T) {
	t.Parallel()

	binName := "ipfs"
	if runtime.GOOS == "windows" {
		binName = "ipfs.exe"
	}

	// Copy the real built binary to a temp directory. Revert will replace
	// this copy with the stash content via os.Executable() -> tmpBinPath.
	tmpBinDir := t.TempDir()
	tmpBinPath := filepath.Join(tmpBinDir, binName)
	copyBuiltBinary(t, tmpBinPath)

	h := harness.NewT(t, func(h *harness.Harness) {
		h.IPFSBin = tmpBinPath
	})
	node := h.NewNode()

	// Create a stash directory with known content that differs from the
	// current binary. findLatestStash looks for ipfs-<semver> files.
	stashDir := filepath.Join(node.Dir, "old-bin")
	require.NoError(t, os.MkdirAll(stashDir, 0o755))
	stashName := "ipfs-0.30.0"
	if runtime.GOOS == "windows" {
		stashName = "ipfs-0.30.0.exe"
	}
	stashPath := filepath.Join(stashDir, stashName)
	stashContent := []byte("#!/bin/sh\necho reverted-to-0.30.0\n")
	require.NoError(t, os.WriteFile(stashPath, stashContent, 0o755))

	// Run: ipfs update revert
	// The subprocess executes from tmpBinPath (a real ipfs binary).
	// os.Executable() returns tmpBinPath, so revert replaces that file
	// with stashContent and removes the stash file.
	res := node.RunIPFS("update", "revert")
	require.NoError(t, res.Err, "revert failed; stderr:\n%s", res.Stderr.String())

	// Verify the revert message.
	assert.Contains(t, res.Stderr.String(), "Reverted to Kubo 0.30.0",
		"should confirm which version was restored")

	// Verify the stash file was cleaned up after successful revert.
	_, err := os.Stat(stashPath)
	assert.True(t, os.IsNotExist(err),
		"stash file should be removed after revert, but still exists at %s", stashPath)

	// Verify the binary was replaced with the stash content.
	got, err := os.ReadFile(tmpBinPath)
	require.NoError(t, err)
	assert.Equal(t, stashContent, got,
		"binary at %s should contain the stash content after revert", tmpBinPath)
}

// --- test helpers ---

// copyBuiltBinary copies the built ipfs binary (cmd/ipfs/ipfs) to dst.
// It locates the project root the same way the test harness does.
func copyBuiltBinary(t *testing.T, dst string) {
	t.Helper()
	// Use a throwaway harness to resolve the default binary path,
	// reusing the same project-root lookup the harness already has.
	h := harness.NewT(t)
	data, err := os.ReadFile(h.IPFSBin)
	require.NoError(t, err, "failed to read built binary at %s (did you run 'make build'?)", h.IPFSBin)
	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	require.NoError(t, os.WriteFile(dst, data, 0o755))
}

// buildTestTarGz creates an in-memory tar.gz archive with a single file entry.
func buildTestTarGz(t *testing.T, path string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: path,
		Mode: 0o755,
		Size: int64(len(content)),
	}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())
	return buf.Bytes()
}

// buildTestZip creates an in-memory zip archive with a single file entry.
func buildTestZip(t *testing.T, path string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Create(path)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}
