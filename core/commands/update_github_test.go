package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- SHA-512 verification ---
//
// These tests verify the integrity-checking code that protects users from
// tampered or corrupted downloads. A broken hash check could allow
// installing a malicious binary, so each failure mode must be covered.

// TestVerifySHA512 exercises the low-level hash comparison function.
func TestVerifySHA512(t *testing.T) {
	t.Parallel()
	data := []byte("hello world")
	sum := sha512.Sum512(data)
	validHex := fmt.Sprintf("%x", sum[:])

	t.Run("accepts matching hash", func(t *testing.T) {
		t.Parallel()
		err := verifySHA512(data, validHex)
		assert.NoError(t, err)
	})

	t.Run("rejects data that does not match hash", func(t *testing.T) {
		t.Parallel()
		err := verifySHA512([]byte("tampered"), validHex)
		assert.ErrorContains(t, err, "SHA-512 mismatch",
			"must reject data whose hash differs from the expected value")
	})

	t.Run("rejects malformed hex string", func(t *testing.T) {
		t.Parallel()
		err := verifySHA512(data, "not-valid-hex")
		assert.ErrorContains(t, err, "invalid hex in SHA-512 checksum")
	})
}

// TestDownloadAndVerifySHA512 tests the complete download-and-verify flow:
// fetching a .sha512 sidecar file from alongside the archive URL, parsing
// the standard sha512sum format ("<hex>  <filename>\n"), and comparing
// against the archive data. This is the function called by "ipfs update install".
func TestDownloadAndVerifySHA512(t *testing.T) {
	t.Parallel()
	archiveData := []byte("fake-archive-content")
	sum := sha512.Sum512(archiveData)
	checksumBody := fmt.Sprintf("%x  kubo_v0.41.0_linux-amd64.tar.gz\n", sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/archive.tar.gz.sha512":
			_, _ = w.Write([]byte(checksumBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	t.Run("accepts archive matching sidecar hash", func(t *testing.T) {
		t.Parallel()
		err := downloadAndVerifySHA512(t.Context(), archiveData, srv.URL+"/archive.tar.gz")
		assert.NoError(t, err)
	})

	t.Run("rejects archive with wrong content", func(t *testing.T) {
		t.Parallel()
		err := downloadAndVerifySHA512(t.Context(), []byte("tampered"), srv.URL+"/archive.tar.gz")
		assert.ErrorContains(t, err, "SHA-512 mismatch",
			"must hard-fail when downloaded archive doesn't match the published checksum")
	})

	t.Run("fails when sidecar file is missing", func(t *testing.T) {
		t.Parallel()
		err := downloadAndVerifySHA512(t.Context(), archiveData, srv.URL+"/no-such-file.tar.gz")
		assert.ErrorContains(t, err, "downloading checksum file",
			"must fail if the .sha512 sidecar can't be fetched")
	})
}

// --- GitHub API layer ---

// TestGitHubGet verifies the low-level GitHub API helper that adds
// authentication headers and translates HTTP errors into actionable
// messages (especially rate-limit hints for unauthenticated users).
func TestGitHubGet(t *testing.T) {
	t.Parallel()

	t.Run("sets Accept and User-Agent headers", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"),
				"must request GitHub's v3 JSON format")
			assert.Contains(t, r.Header.Get("User-Agent"), "kubo/",
				"User-Agent must identify the kubo version for debugging")
			_, _ = w.Write([]byte("{}"))
		}))
		t.Cleanup(srv.Close)

		resp, err := githubGet(t.Context(), srv.URL)
		require.NoError(t, err)
		resp.Body.Close()
	})

	t.Run("returns rate-limit error on HTTP 403", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		t.Cleanup(srv.Close)

		_, err := githubGet(t.Context(), srv.URL)
		assert.ErrorContains(t, err, "rate limit exceeded")
	})

	t.Run("returns rate-limit error on HTTP 429", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		t.Cleanup(srv.Close)

		_, err := githubGet(t.Context(), srv.URL)
		assert.ErrorContains(t, err, "rate limit exceeded")
	})

	t.Run("returns HTTP status on server error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		_, err := githubGet(t.Context(), srv.URL)
		assert.ErrorContains(t, err, "HTTP 500")
	})
}

// TestGitHubListReleases verifies that release listing correctly filters
// prereleases and respects the count limit. Uses a mock GitHub API server
// to avoid network dependencies and rate limits in CI.
//
// Not parallel: temporarily overrides the package-level githubReleaseFmt var.
func TestGitHubListReleases(t *testing.T) {
	allReleases := []ghRelease{
		{TagName: "v0.42.0-rc1", Prerelease: true},
		{TagName: "v0.41.0"},
		{TagName: "v0.40.0"},
	}
	body, err := json.Marshal(allReleases)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	saved := githubReleaseFmt
	githubReleaseFmt = srv.URL
	t.Cleanup(func() { githubReleaseFmt = saved })

	t.Run("excludes prereleases by default", func(t *testing.T) {
		got, err := githubListReleases(t.Context(), 10, false)
		require.NoError(t, err)
		assert.Len(t, got, 2, "the rc1 prerelease should be filtered out")
		assert.Equal(t, "v0.41.0", got[0].TagName)
		assert.Equal(t, "v0.40.0", got[1].TagName)
	})

	t.Run("includes prereleases when requested", func(t *testing.T) {
		got, err := githubListReleases(t.Context(), 10, true)
		require.NoError(t, err)
		assert.Len(t, got, 3)
		assert.Equal(t, "v0.42.0-rc1", got[0].TagName)
	})

	t.Run("respects count limit", func(t *testing.T) {
		got, err := githubListReleases(t.Context(), 1, false)
		require.NoError(t, err)
		assert.Len(t, got, 1, "should return at most 1 release")
	})
}

// TestGitHubLatestRelease verifies that the "find latest release" logic
// skips releases that don't have a binary for the current OS/arch.
// This handles the real-world case where a release tag is created but
// CI hasn't finished uploading build artifacts yet.
//
// Not parallel: temporarily overrides the package-level githubReleaseFmt var.
func TestGitHubLatestRelease(t *testing.T) {
	releases := []ghRelease{
		{
			TagName: "v0.42.0",
			Assets:  []ghAsset{{Name: "kubo_v0.42.0_some-other-arch.tar.gz"}},
		},
		{
			TagName: "v0.41.0",
			Assets:  []ghAsset{{Name: assetNameForPlatformTag("v0.41.0")}},
		},
	}
	body, err := json.Marshal(releases)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	saved := githubReleaseFmt
	githubReleaseFmt = srv.URL
	t.Cleanup(func() { githubReleaseFmt = saved })

	rel, err := githubLatestRelease(t.Context(), false)
	require.NoError(t, err)
	assert.Equal(t, "v0.41.0", rel.TagName,
		"should skip v0.42.0 (no binary for %s/%s) and return v0.41.0",
		runtime.GOOS, runtime.GOARCH)
}

// TestFindReleaseAsset verifies that findReleaseAsset locates the correct
// platform-specific asset in a release, and returns a clear error when the
// release exists but has no binary for the current OS/arch.
//
// Not parallel: temporarily overrides the package-level githubReleaseFmt var.
func TestFindReleaseAsset(t *testing.T) {
	wantAsset := assetNameForPlatformTag("v0.50.0")

	release := ghRelease{
		TagName: "v0.50.0",
		Assets: []ghAsset{
			{Name: "kubo_v0.50.0_some-other-arch.tar.gz", BrowserDownloadURL: "https://example.com/other"},
			{Name: wantAsset, BrowserDownloadURL: "https://example.com/correct"},
		},
	}
	body, err := json.Marshal(release)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	saved := githubReleaseFmt
	githubReleaseFmt = srv.URL
	t.Cleanup(func() { githubReleaseFmt = saved })

	t.Run("returns matching asset for current platform", func(t *testing.T) {
		rel, asset, err := findReleaseAsset(t.Context(), "v0.50.0")
		require.NoError(t, err)
		assert.Equal(t, "v0.50.0", rel.TagName)
		assert.Equal(t, wantAsset, asset.Name)
		assert.Equal(t, "https://example.com/correct", asset.BrowserDownloadURL)
	})

	t.Run("returns error when no asset matches current platform", func(t *testing.T) {
		// Serve a release that only has an asset for a different arch.
		noMatch := ghRelease{
			TagName: "v0.51.0",
			Assets:  []ghAsset{{Name: "kubo_v0.51.0_plan9-mips.tar.gz"}},
		}
		noMatchBody, err := json.Marshal(noMatch)
		require.NoError(t, err)

		noMatchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(noMatchBody)
		}))
		t.Cleanup(noMatchSrv.Close)

		githubReleaseFmt = noMatchSrv.URL

		_, _, err = findReleaseAsset(t.Context(), "v0.51.0")
		assert.ErrorContains(t, err, "has no binary for",
			"should explain that the release exists but lacks a matching asset")
	})
}

// --- Asset download ---

// TestDownloadAsset verifies the HTTP download helper that fetches release
// archives from GitHub's CDN. Tests both the happy path and HTTP error
// reporting.
func TestDownloadAsset(t *testing.T) {
	t.Parallel()

	t.Run("downloads content successfully", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("binary-content"))
		}))
		t.Cleanup(srv.Close)

		data, err := downloadAsset(t.Context(), srv.URL)
		require.NoError(t, err)
		assert.Equal(t, []byte("binary-content"), data)
	})

	t.Run("returns clear error on HTTP failure", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)

		_, err := downloadAsset(t.Context(), srv.URL)
		assert.ErrorContains(t, err, "HTTP 404")
	})
}

// --- Archive extraction ---

// TestExtractBinaryFromArchive verifies that the ipfs binary can be
// extracted from release archives. Kubo releases use tar.gz on Unix
// and zip on Windows, with the binary at "kubo/ipfs" inside the archive.
func TestExtractBinaryFromArchive(t *testing.T) {
	t.Parallel()

	t.Run("extracts binary from valid tar.gz", func(t *testing.T) {
		t.Parallel()
		wantContent := []byte("#!/bin/fake-ipfs-binary")
		archive := makeTarGz(t, "kubo/ipfs", wantContent)

		got, err := extractBinaryFromArchive(archive)
		require.NoError(t, err)
		assert.Equal(t, wantContent, got)
	})

	t.Run("rejects archive without kubo/ipfs entry", func(t *testing.T) {
		t.Parallel()
		// A valid tar.gz that contains a file at the wrong path.
		archive := makeTarGz(t, "wrong-path/ipfs", []byte("binary"))

		_, err := extractBinaryFromArchive(archive)
		assert.ErrorContains(t, err, "could not find ipfs binary")
	})

	t.Run("rejects non-archive data", func(t *testing.T) {
		t.Parallel()
		_, err := extractBinaryFromArchive([]byte("not an archive"))
		assert.ErrorContains(t, err, "could not find ipfs binary")
	})
}

// makeTarGz creates an in-memory tar.gz archive containing a single file.
func makeTarGz(t *testing.T, path string, content []byte) []byte {
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

// --- Asset name and version helpers ---

// TestAssetNameForPlatformTag ensures the archive filename matches the
// naming convention used by Kubo's CI release pipeline:
//
//	kubo_<tag>_<os>-<arch>.<ext>
func TestAssetNameForPlatformTag(t *testing.T) {
	t.Parallel()
	name := assetNameForPlatformTag("v0.41.0")
	assert.Contains(t, name, fmt.Sprintf("kubo_v0.41.0_%s-%s.", runtime.GOOS, runtime.GOARCH))

	if runtime.GOOS == "windows" {
		assert.Contains(t, name, ".zip")
	} else {
		assert.Contains(t, name, ".tar.gz")
	}
}

// TestVersionHelpers exercises the version string utilities used throughout
// the update command. These handle the mismatch between Go's semver
// (no "v" prefix) and GitHub's tag convention ("v" prefix).
func TestVersionHelpers(t *testing.T) {
	t.Parallel()

	t.Run("trimVPrefix strips leading v", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "0.41.0", trimVPrefix("v0.41.0"))
		assert.Equal(t, "0.41.0", trimVPrefix("0.41.0"), "no-op when v is absent")
	})

	t.Run("normalizeVersion adds v prefix for GitHub tags", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "v0.41.0", normalizeVersion("0.41.0"))
		assert.Equal(t, "v0.41.0", normalizeVersion("v0.41.0"), "no-op when v is present")
		assert.Equal(t, "v0.41.0", normalizeVersion(" v0.41.0 "), "trims whitespace")
	})

	t.Run("isNewerVersion compares semver correctly", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			current, target string
			wantNewer       bool
			desc            string
		}{
			{"0.40.0", "0.41.0", true, "newer minor version"},
			{"0.41.0", "0.40.0", false, "older minor version"},
			{"0.41.0", "0.41.0", false, "same version"},
			{"0.41.0-dev", "0.41.0", true, "release is newer than dev pre-release"},
		}
		for _, tt := range tests {
			got, err := isNewerVersion(tt.current, tt.target)
			require.NoError(t, err)
			assert.Equal(t, tt.wantNewer, got, tt.desc)
		}
	})
}
