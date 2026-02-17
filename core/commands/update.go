package commands

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	goversion "github.com/hashicorp/go-version"
	cmds "github.com/ipfs/go-ipfs-cmds"
	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/ipfs/kubo/repo/fsrepo/migrations"
	"github.com/ipfs/kubo/repo/fsrepo/migrations/atomicfile"
)

const (
	updatePreOptionName            = "pre"
	updateCountOptionName          = "count"
	updateAllowDowngradeOptionName = "allow-downgrade"
)

// UpdateCmd is the "ipfs update" command tree.
var UpdateCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Update Kubo to a different version",
		ShortDescription: `
Downloads pre-built Kubo binaries from GitHub Releases, verifies
checksums, and replaces the running binary in place. The previous
binary is saved so you can revert if needed.

The daemon must be stopped before installing or reverting.
`,
		LongDescription: `
Downloads pre-built Kubo binaries from GitHub Releases, verifies
checksums, and replaces the running binary in place. The previous
binary is saved so you can revert if needed.

The daemon must be stopped before installing or reverting.

ENVIRONMENT VARIABLES

  HTTPS_PROXY
      HTTP proxy for reaching GitHub. Set this when GitHub is not
      directly reachable from your network.
      Example: HTTPS_PROXY=http://proxy:8080 ipfs update install

  GITHUB_TOKEN
      GitHub personal access token. Raises the API rate limit from
      60 to 5000 requests per hour. Set this if you hit "rate limit
      exceeded" errors. GH_TOKEN is also accepted.

  IPFS_PATH
      Determines where binary backups are stored ($IPFS_PATH/old-bin/).
      Defaults to ~/.ipfs.
`,
	},
	NoRemote: true,
	Extra:    CreateCmdExtras(SetDoesNotUseRepo(true), SetDoesNotUseConfigAsInput(true)),
	Subcommands: map[string]*cmds.Command{
		"check":    updateCheckCmd,
		"versions": updateVersionsCmd,
		"install":  updateInstallCmd,
		"revert":   updateRevertCmd,
	},
}

// -- check --

// UpdateCheckOutput is the output of "ipfs update check".
type UpdateCheckOutput struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
}

var updateCheckCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Check if a newer Kubo version is available",
		ShortDescription: `
Queries GitHub Releases for the latest Kubo version and compares
it against the currently running binary. Only considers releases
with binaries available for your operating system and architecture.

Works while the daemon is running (read-only, no repo access).

ENVIRONMENT VARIABLES

  HTTPS_PROXY   HTTP proxy for reaching GitHub API.
  GITHUB_TOKEN  Raises the API rate limit (GH_TOKEN also accepted).
`,
	},
	NoRemote: true,
	Extra:    CreateCmdExtras(SetDoesNotUseRepo(true), SetDoesNotUseConfigAsInput(true)),
	Options: []cmds.Option{
		cmds.BoolOption(updatePreOptionName, "Include pre-release versions."),
	},
	Type: UpdateCheckOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		includePre, _ := req.Options[updatePreOptionName].(bool)

		rel, err := githubLatestRelease(ctx, includePre)
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		latest := trimVPrefix(rel.TagName)
		current := currentVersion()

		updateAvailable, err := isNewerVersion(current, latest)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &UpdateCheckOutput{
			CurrentVersion:  current,
			LatestVersion:   latest,
			UpdateAvailable: updateAvailable,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *UpdateCheckOutput) error {
			if out.UpdateAvailable {
				fmt.Fprintf(w, "Update available: %s -> %s\n", out.CurrentVersion, out.LatestVersion)
				fmt.Fprintln(w, "Run 'ipfs update install' to install the latest version.")
			} else {
				fmt.Fprintf(w, "Already up to date (%s)\n", out.CurrentVersion)
			}
			return nil
		}),
	},
}

// -- versions --

// UpdateVersionsOutput is the output of "ipfs update versions".
type UpdateVersionsOutput struct {
	Current  string
	Versions []string
}

var updateVersionsCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "List available Kubo versions",
		ShortDescription: `
Lists Kubo versions published on GitHub Releases. The currently
running version is marked with an asterisk (*).
`,
	},
	NoRemote: true,
	Extra:    CreateCmdExtras(SetDoesNotUseRepo(true), SetDoesNotUseConfigAsInput(true)),
	Options: []cmds.Option{
		cmds.IntOption(updateCountOptionName, "n", "Number of versions to list.").WithDefault(30),
		cmds.BoolOption(updatePreOptionName, "Include pre-release versions."),
	},
	Type: UpdateVersionsOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		count, _ := req.Options[updateCountOptionName].(int)
		if count <= 0 {
			count = 30
		}
		includePre, _ := req.Options[updatePreOptionName].(bool)

		releases, err := githubListReleases(ctx, count, includePre)
		if err != nil {
			return fmt.Errorf("listing versions: %w", err)
		}

		versions := make([]string, 0, len(releases))
		for _, r := range releases {
			versions = append(versions, trimVPrefix(r.TagName))
		}

		return cmds.EmitOnce(res, &UpdateVersionsOutput{
			Current:  currentVersion(),
			Versions: versions,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *UpdateVersionsOutput) error {
			for _, v := range out.Versions {
				marker := "  "
				if v == out.Current {
					marker = "* "
				}
				fmt.Fprintf(w, "%s%s\n", marker, v)
			}
			return nil
		}),
	},
}

// -- install --

// UpdateInstallOutput is the output of "ipfs update install".
type UpdateInstallOutput struct {
	OldVersion string
	NewVersion string
	BinaryPath string
	StashedTo  string
}

var updateInstallCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Download and install a Kubo update",
		ShortDescription: `
Downloads the specified version (or latest) from GitHub Releases,
verifies the SHA-512 checksum, saves a backup of the current binary,
and atomically replaces it.

If replacing the binary fails due to file permissions, the new binary
is saved to a temporary directory and the path is printed so you can
move it manually (e.g. with sudo).

Previous binaries are kept in $IPFS_PATH/old-bin/ and can be
restored with 'ipfs update revert'.
`,
	},
	NoRemote: true,
	Extra:    CreateCmdExtras(SetDoesNotUseRepo(true), SetDoesNotUseConfigAsInput(true)),
	Arguments: []cmds.Argument{
		cmds.StringArg("version", false, false, "Version to install (default: latest)."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(updatePreOptionName, "Include pre-release versions when resolving latest."),
		cmds.BoolOption(updateAllowDowngradeOptionName, "Allow installing an older version."),
	},
	Type: UpdateInstallOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		if err := checkDaemonNotRunning(); err != nil {
			return err
		}

		current := currentVersion()
		includePre, _ := req.Options[updatePreOptionName].(bool)
		allowDowngrade, _ := req.Options[updateAllowDowngradeOptionName].(bool)

		// Resolve target version.
		var tag string
		if len(req.Arguments) > 0 && req.Arguments[0] != "" {
			tag = normalizeVersion(req.Arguments[0])
		} else {
			rel, err := githubLatestRelease(ctx, includePre)
			if err != nil {
				return fmt.Errorf("finding latest release: %w", err)
			}
			tag = rel.TagName
		}
		target := trimVPrefix(tag)

		// Compare versions.
		if target == current {
			return fmt.Errorf("already running version %s", current)
		}

		newer, err := isNewerVersion(current, target)
		if err != nil {
			return err
		}
		if !newer && !allowDowngrade {
			return fmt.Errorf("version %s is older than current %s (use --allow-downgrade to force)", target, current)
		}

		// Find and download asset.
		fmt.Fprintf(os.Stderr, "Downloading Kubo %s...\n", target)

		_, asset, err := findReleaseAsset(ctx, normalizeVersion(target))
		if err != nil {
			return err
		}

		data, err := downloadAsset(ctx, asset.BrowserDownloadURL)
		if err != nil {
			return err
		}

		// Verify checksum using .sha512 sidecar file.
		if err := downloadAndVerifySHA512(ctx, data, asset.BrowserDownloadURL); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "Checksum verified (SHA-512).")

		// Extract binary from archive.
		binData, err := extractBinaryFromArchive(data)
		if err != nil {
			return fmt.Errorf("extracting binary: %w", err)
		}

		// Resolve current binary path.
		binPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("finding current binary: %w", err)
		}
		binPath, err = filepath.EvalSymlinks(binPath)
		if err != nil {
			return fmt.Errorf("resolving binary path: %w", err)
		}

		// Stash current binary.
		stashedTo, err := stashBinary(binPath, current)
		if err != nil {
			return fmt.Errorf("backing up current binary: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Backed up current binary to %s\n", stashedTo)

		// Replace binary.
		if err := replaceBinary(binPath, binData); err != nil {
			// Permission error fallback: save to temp dir.
			if errors.Is(err, os.ErrPermission) {
				tmpPath := filepath.Join(os.TempDir(), migrations.ExeName(fmt.Sprintf("ipfs-%s", target)))
				if writeErr := os.WriteFile(tmpPath, binData, 0o755); writeErr != nil {
					return fmt.Errorf("cannot write to %s either: %w (original error: %v)", tmpPath, writeErr, err)
				}
				fmt.Fprintf(os.Stderr, "Could not replace %s (permission denied).\n", binPath)
				fmt.Fprintf(os.Stderr, "New binary saved to: %s\n", tmpPath)
				fmt.Fprintf(os.Stderr, "Move it manually, e.g.: sudo mv %s %s\n", tmpPath, binPath)
				return cmds.EmitOnce(res, &UpdateInstallOutput{
					OldVersion: current,
					NewVersion: target,
					BinaryPath: tmpPath,
					StashedTo:  stashedTo,
				})
			}
			return fmt.Errorf("replacing binary: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Successfully updated Kubo %s -> %s\n", current, target)

		return cmds.EmitOnce(res, &UpdateInstallOutput{
			OldVersion: current,
			NewVersion: target,
			BinaryPath: binPath,
			StashedTo:  stashedTo,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *UpdateInstallOutput) error {
			// All status output goes to stderr in Run; text encoder is a no-op.
			return nil
		}),
	},
}

// -- revert --

// UpdateRevertOutput is the output of "ipfs update revert".
type UpdateRevertOutput struct {
	RestoredVersion string
	BinaryPath      string
}

var updateRevertCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Revert to a previously installed Kubo version",
		ShortDescription: `
Restores the most recently backed up binary from $IPFS_PATH/old-bin/.
The backup is created automatically by 'ipfs update install'.
`,
	},
	NoRemote: true,
	Extra:    CreateCmdExtras(SetDoesNotUseRepo(true), SetDoesNotUseConfigAsInput(true)),
	Type:     UpdateRevertOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		if err := checkDaemonNotRunning(); err != nil {
			return err
		}

		stashDir, err := getStashDir()
		if err != nil {
			return err
		}

		stashPath, stashVer, err := findLatestStash(stashDir)
		if err != nil {
			return err
		}

		stashData, err := os.ReadFile(stashPath)
		if err != nil {
			return fmt.Errorf("reading stashed binary: %w", err)
		}

		binPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("finding current binary: %w", err)
		}
		binPath, err = filepath.EvalSymlinks(binPath)
		if err != nil {
			return fmt.Errorf("resolving binary path: %w", err)
		}

		if err := replaceBinary(binPath, stashData); err != nil {
			if errors.Is(err, os.ErrPermission) {
				tmpPath := filepath.Join(os.TempDir(), migrations.ExeName(fmt.Sprintf("ipfs-%s", stashVer)))
				if writeErr := os.WriteFile(tmpPath, stashData, 0o755); writeErr != nil {
					return fmt.Errorf("cannot write to %s either: %w (original error: %v)", tmpPath, writeErr, err)
				}
				fmt.Fprintf(os.Stderr, "Could not replace %s (permission denied).\n", binPath)
				fmt.Fprintf(os.Stderr, "Reverted binary saved to: %s\n", tmpPath)
				fmt.Fprintf(os.Stderr, "Move it manually, e.g.: sudo mv %s %s\n", tmpPath, binPath)
				return cmds.EmitOnce(res, &UpdateRevertOutput{
					RestoredVersion: stashVer,
					BinaryPath:      tmpPath,
				})
			}
			return fmt.Errorf("replacing binary: %w", err)
		}

		// Remove the stash file that was restored.
		os.Remove(stashPath)

		fmt.Fprintf(os.Stderr, "Reverted to Kubo %s\n", stashVer)

		return cmds.EmitOnce(res, &UpdateRevertOutput{
			RestoredVersion: stashVer,
			BinaryPath:      binPath,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *UpdateRevertOutput) error {
			return nil
		}),
	},
}

// -- helpers --

// currentVersion returns the version string used by update commands.
// It defaults to version.CurrentVersionNumber but can be overridden by
// setting IPFS_VERSION_FAKE, which is useful for testing update
// detection without rebuilding the binary.
func currentVersion() string {
	if v := os.Getenv("IPFS_VERSION_FAKE"); v != "" {
		return v
	}
	return version.CurrentVersionNumber
}

// checkDaemonNotRunning returns an error if the IPFS daemon is running.
func checkDaemonNotRunning() error {
	repoPath, err := fsrepo.BestKnownPath()
	if err != nil {
		// If we can't determine the repo path, skip the check.
		return nil
	}
	locked, err := fsrepo.LockedByOtherProcess(repoPath)
	if err != nil {
		// Lock check failed (e.g. repo doesn't exist yet), not an error.
		return nil
	}
	if locked {
		return fmt.Errorf("IPFS daemon is running (repo locked at %s). Stop it first with 'ipfs shutdown'", repoPath)
	}
	return nil
}

// getStashDir returns the path to the old-bin stash directory, creating it if needed.
func getStashDir() (string, error) {
	repoPath, err := fsrepo.BestKnownPath()
	if err != nil {
		return "", fmt.Errorf("determining IPFS path: %w", err)
	}
	dir := filepath.Join(repoPath, "old-bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating stash directory: %w", err)
	}
	return dir, nil
}

// stashBinary copies the current binary to the stash directory.
// Uses named returns so the deferred dst.Close() error is not silently
// discarded -- a failed close means the backup may be incomplete.
func stashBinary(binPath, ver string) (stashPath string, err error) {
	dir, err := getStashDir()
	if err != nil {
		return "", err
	}

	stashName := migrations.ExeName(fmt.Sprintf("ipfs-%s", ver))
	stashPath = filepath.Join(dir, stashName)

	src, err := os.Open(binPath)
	if err != nil {
		return "", fmt.Errorf("opening current binary: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(stashPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating stash file: %w", err)
	}
	defer func() {
		if cerr := dst.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("writing stash file: %w", cerr)
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("copying binary to stash: %w", err)
	}

	return stashPath, nil
}

// findLatestStash finds the most recently versioned stash file.
func findLatestStash(dir string) (path, ver string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("reading stash directory: %w", err)
	}

	type stash struct {
		path   string
		ver    string
		parsed *goversion.Version
	}

	var stashes []stash
	for _, e := range entries {
		name := e.Name()
		// Expected format: ipfs-<version> or ipfs-<version>.exe
		trimmed := strings.TrimPrefix(name, "ipfs-")
		if trimmed == name {
			continue // doesn't match pattern
		}
		trimmed = strings.TrimSuffix(trimmed, ".exe")
		parsed, parseErr := goversion.NewVersion(trimmed)
		if parseErr != nil {
			continue
		}
		stashes = append(stashes, stash{
			path:   filepath.Join(dir, name),
			ver:    trimmed,
			parsed: parsed,
		})
	}

	if len(stashes) == 0 {
		return "", "", fmt.Errorf("no stashed binaries found in %s", dir)
	}

	slices.SortFunc(stashes, func(a, b stash) int {
		// Sort newest first: if a > b return -1.
		if a.parsed.GreaterThan(b.parsed) {
			return -1
		}
		if b.parsed.GreaterThan(a.parsed) {
			return 1
		}
		return 0
	})

	return stashes[0].path, stashes[0].ver, nil
}

// replaceBinary atomically replaces the binary at targetPath with data.
func replaceBinary(targetPath string, data []byte) error {
	af, err := atomicfile.New(targetPath, 0o755)
	if err != nil {
		return err
	}

	if _, err := af.Write(data); err != nil {
		_ = af.Abort()
		return err
	}

	return af.Close()
}

// extractBinaryFromArchive extracts the kubo/ipfs binary from a tar.gz or zip archive.
func extractBinaryFromArchive(data []byte) ([]byte, error) {
	binName := migrations.ExeName("ipfs")

	// Try tar.gz first, then zip.
	if result, err := extractFromTarGz(data, binName); err == nil {
		return result, nil
	}

	if result, err := extractFromZip(data, binName); err == nil {
		return result, nil
	}

	return nil, errors.New("could not find ipfs binary in archive (expected kubo/ipfs)")
}

func extractFromTarGz(data []byte, binName string) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	lookFor := "kubo/" + binName
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == lookFor {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%s not found in tar.gz", lookFor)
}

func extractFromZip(data []byte, binName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	lookFor := "kubo/" + binName
	for _, f := range zr.File {
		if f.Name != lookFor {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		result, err := io.ReadAll(rc)
		rc.Close()
		return result, err
	}
	return nil, fmt.Errorf("%s not found in zip", lookFor)
}

// trimVPrefix removes a leading "v" from a version string.
func trimVPrefix(s string) string {
	return strings.TrimPrefix(s, "v")
}

// normalizeVersion ensures a version string has a "v" prefix (for GitHub tags).
func normalizeVersion(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "v") {
		return "v" + s
	}
	return s
}

// isNewerVersion returns true if target is newer than current.
func isNewerVersion(current, target string) (bool, error) {
	cv, err := goversion.NewVersion(current)
	if err != nil {
		return false, fmt.Errorf("parsing current version %q: %w", current, err)
	}
	tv, err := goversion.NewVersion(target)
	if err != nil {
		return false, fmt.Errorf("parsing target version %q: %w", target, err)
	}
	return tv.GreaterThan(cv), nil
}
