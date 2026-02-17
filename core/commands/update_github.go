package commands

// This file implements fetching Kubo release binaries from GitHub Releases.
//
// We use GitHub Releases instead of dist.ipfs.tech because GitHub is harder
// to censor. Many networks and regions block or interfere with IPFS-specific
// infrastructure, but GitHub is widely accessible and its TLS-protected API
// is difficult to selectively block without breaking many other services.

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	version "github.com/ipfs/kubo"
)

const (
	githubOwner = "ipfs"
	githubRepo  = "kubo"

	githubAPIBase = "https://api.github.com"

	// maxDownloadSize is the maximum allowed binary archive size (200 MB).
	maxDownloadSize = 200 << 20
)

// githubReleaseFmt is the default GitHub Releases API URL prefix.
// It is a var (not const) so unit tests can point API calls at a mock server.
var githubReleaseFmt = githubAPIBase + "/repos/" + githubOwner + "/" + githubRepo + "/releases"

// githubReleaseBaseURL returns the Releases API base URL.
// It checks KUBO_UPDATE_GITHUB_URL first (used by CLI integration tests),
// then falls back to githubReleaseFmt (overridable by unit tests).
func githubReleaseBaseURL() string {
	if u := os.Getenv("KUBO_UPDATE_GITHUB_URL"); u != "" {
		return u
	}
	return githubReleaseFmt
}

// ghRelease represents a GitHub release.
type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

// ghAsset represents a release asset on GitHub.
type ghAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// githubGet performs an authenticated GET request to the GitHub API.
// It honors GITHUB_TOKEN or GH_TOKEN env vars to avoid the 60 req/hr
// unauthenticated rate limit.
func githubGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kubo/"+version.CurrentVersionNumber)

	if token := githubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		hint := ""
		if githubToken() == "" {
			hint = " (hint: set GITHUB_TOKEN or GH_TOKEN to avoid rate limits)"
		}
		return nil, fmt.Errorf("GitHub API rate limit exceeded%s", hint)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GitHub API returned HTTP %d for %s", resp.StatusCode, url)
	}

	return resp, nil
}

func githubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}

// githubLatestRelease returns the newest release that has a platform asset
// for the current GOOS/GOARCH. This avoids false positives when a release
// tag exists but artifacts haven't been uploaded yet.
func githubLatestRelease(ctx context.Context, includePre bool) (*ghRelease, error) {
	releases, err := githubListReleases(ctx, 10, includePre)
	if err != nil {
		return nil, err
	}

	for i := range releases {
		want := assetNameForPlatformTag(releases[i].TagName)
		for _, a := range releases[i].Assets {
			if a.Name == want {
				return &releases[i], nil
			}
		}
	}
	return nil, fmt.Errorf("no release found with a binary for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// githubListReleases fetches up to count releases, optionally including prereleases.
func githubListReleases(ctx context.Context, count int, includePre bool) ([]ghRelease, error) {
	// Fetch more than needed so we can filter prereleases and still return count results.
	perPage := count
	if !includePre {
		perPage = count * 3
	}
	if perPage > 100 {
		perPage = 100
	}

	url := fmt.Sprintf("%s?per_page=%d", githubReleaseBaseURL(), perPage)
	resp, err := githubGet(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var all []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, fmt.Errorf("decoding GitHub releases: %w", err)
	}

	var filtered []ghRelease
	for _, r := range all {
		if !includePre && r.Prerelease {
			continue
		}
		filtered = append(filtered, r)
		if len(filtered) >= count {
			break
		}
	}
	return filtered, nil
}

// githubReleaseByTag fetches a single release by its git tag.
func githubReleaseByTag(ctx context.Context, tag string) (*ghRelease, error) {
	url := fmt.Sprintf("%s/tags/%s", githubReleaseBaseURL(), tag)
	resp, err := githubGet(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding GitHub release: %w", err)
	}
	return &rel, nil
}

// findReleaseAsset locates the platform-appropriate asset in a release.
// It fails immediately with a clear message if:
//   - the release tag does not exist on GitHub (typo, unreleased version)
//   - the release exists but has no binary for this OS/arch (CI still building)
func findReleaseAsset(ctx context.Context, tag string) (*ghRelease, *ghAsset, error) {
	rel, err := githubReleaseByTag(ctx, tag)
	if err != nil {
		return nil, nil, fmt.Errorf("release %s not found on GitHub: %w", tag, err)
	}

	want := assetNameForPlatformTag(tag)
	for i := range rel.Assets {
		if rel.Assets[i].Name == want {
			return rel, &rel.Assets[i], nil
		}
	}

	return nil, nil, fmt.Errorf(
		"release %s exists but has no binary for %s/%s yet; build artifacts may still be uploading, try again in a few hours",
		tag, runtime.GOOS, runtime.GOARCH)
}

// downloadAsset downloads a release asset by its browser_download_url.
// This hits GitHub's CDN directly, not the API, so no auth headers are needed.
func downloadAsset(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "kubo/"+version.CurrentVersionNumber)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading download: %w", err)
	}
	if int64(len(data)) > maxDownloadSize {
		return nil, fmt.Errorf("download exceeds maximum size of %d bytes", maxDownloadSize)
	}
	return data, nil
}

// downloadAndVerifySHA512 downloads the .sha512 sidecar file for the given
// archive URL and verifies the archive data against it.
func downloadAndVerifySHA512(ctx context.Context, data []byte, archiveURL string) error {
	sha512URL := archiveURL + ".sha512"
	checksumData, err := downloadAsset(ctx, sha512URL)
	if err != nil {
		return fmt.Errorf("downloading checksum file: %w", err)
	}

	// Parse "<hex>  <filename>\n" format (standard sha512sum output).
	fields := strings.Fields(string(checksumData))
	if len(fields) < 1 {
		return fmt.Errorf("empty or malformed .sha512 file")
	}
	wantHex := fields[0]

	return verifySHA512(data, wantHex)
}

// verifySHA512 checks that data matches the given hex-encoded SHA-512 hash.
func verifySHA512(data []byte, wantHex string) error {
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		return fmt.Errorf("invalid hex in SHA-512 checksum: %w", err)
	}
	got := sha512.Sum512(data)
	if !bytes.Equal(got[:], want) {
		return fmt.Errorf("SHA-512 mismatch: expected %s, got %x", wantHex, got[:])
	}
	return nil
}

// assetNameForPlatformTag returns the expected archive filename for a given
// release tag and the current GOOS/GOARCH.
func assetNameForPlatformTag(tag string) string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("kubo_%s_%s-%s.%s", tag, runtime.GOOS, runtime.GOARCH, ext)
}
