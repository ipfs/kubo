package migrations

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DownloadDirectory can be set as the location for FetchBinary to save the
// downloaded archive file in.  If not set, then FetchBinary saves the archive
// in a temporary directory that is removed after the contents of the archive
// is extracted.
var DownloadDirectory string

// FetchBinary downloads an archive from the distribution site and unpacks it.
//
// The base name of the binary inside the archive may differ from the base
// archive name.  If it does, then specify binName.  For example, the following
// is needed because the archive "go-ipfs_v0.7.0_linux-amd64.tar.gz" contains a
// binary named "ipfs"
//
//	FetchBinary(ctx, fetcher, "go-ipfs", "v0.7.0", "ipfs", tmpDir)
//
// If out is a directory, then the binary is written to that directory with the
// same name it has inside the archive.  Otherwise, the binary file is written
// to the file named by out.
func FetchBinary(ctx context.Context, fetcher Fetcher, dist, ver, binName, out string) (string, error) {
	// The archive file name is the base of dist. This is to support a possible subdir in
	// dist, for example: "ipfs-repo-migrations/fs-repo-11-to-12"
	arcName := filepath.Base(dist)
	// If binary base name is not specified, then it is same as archive base name.
	if binName == "" {
		binName = arcName
	}

	// Name of binary that exists inside archive
	binName = ExeName(binName)

	// Return error if file exists or stat fails for reason other than not
	// exists.  If out is a directory, then write extracted binary to that dir.
	fi, err := os.Stat(out)
	if !os.IsNotExist(err) {
		if err != nil {
			return "", err
		}
		if !fi.IsDir() {
			return "", &os.PathError{
				Op:   "FetchBinary",
				Path: out,
				Err:  os.ErrExist,
			}
		}
		// out exists and is a directory, so compose final name
		out = filepath.Join(out, binName)
		// Check if the binary already exists in the directory
		_, err = os.Stat(out)
		if !os.IsNotExist(err) {
			if err != nil {
				return "", err
			}
			return "", &os.PathError{
				Op:   "FetchBinary",
				Path: out,
				Err:  os.ErrExist,
			}
		}
	}

	tmpDir := DownloadDirectory
	if tmpDir != "" {
		fi, err = os.Stat(tmpDir)
		if err != nil {
			return "", err
		}
		if !fi.IsDir() {
			return "", &os.PathError{
				Op:   "FetchBinary",
				Path: tmpDir,
				Err:  os.ErrExist,
			}
		}
	} else {
		// Create temp directory to store download
		tmpDir, err = os.MkdirTemp("", arcName)
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tmpDir)
	}

	atype := "tar.gz"
	if runtime.GOOS == "windows" {
		atype = "zip"
	}

	arcDistPath, arcFullName := makeArchivePath(dist, arcName, ver, atype)

	// Create a file to write the archive data to
	arcPath := filepath.Join(tmpDir, arcFullName)
	arcFile, err := os.Create(arcPath)
	if err != nil {
		return "", err
	}
	defer arcFile.Close()

	// Open connection to download archive from ipfs path and write to file
	arcBytes, err := fetcher.Fetch(ctx, arcDistPath)
	if err != nil {
		return "", err
	}

	// Write download data
	_, err = io.Copy(arcFile, bytes.NewReader(arcBytes))
	if err != nil {
		return "", err
	}
	arcFile.Close()

	// Unpack the archive and write binary to out
	err = unpackArchive(arcPath, atype, dist, binName, out)
	if err != nil {
		return "", err
	}

	// Set mode of binary to executable
	err = os.Chmod(out, 0o755)
	if err != nil {
		return "", err
	}

	return out, nil
}

// osWithVariant returns the OS name with optional variant.
// Currently returns either runtime.GOOS, or "linux-musl".
func osWithVariant() (string, error) {
	if runtime.GOOS != "linux" {
		return runtime.GOOS, nil
	}

	// ldd outputs the system's kind of libc.
	// - on standard ubuntu: ldd (Ubuntu GLIBC 2.23-0ubuntu5) 2.23
	// - on alpine: musl libc (x86_64)
	//
	// we use the combined stdout+stderr,
	// because ldd --version prints differently on different OSes.
	// - on standard ubuntu: stdout
	// - on alpine: stderr (it probably doesn't know the --version flag)
	//
	// we suppress non-zero exit codes (see last point about alpine).
	out, err := exec.Command("sh", "-c", "ldd --version || true").CombinedOutput()
	if err != nil {
		return "", err
	}

	// now just see if we can find "musl" somewhere in the output
	scan := bufio.NewScanner(bytes.NewBuffer(out))
	for scan.Scan() {
		if strings.Contains(scan.Text(), "musl") {
			return "linux-musl", nil
		}
	}

	return "linux", nil
}

// makeArchivePath composes the path, relative to the distribution site, from which to
// download a binary.  The path returned does not contain the distribution site path,
// e.g. "/ipns/dist.ipfs.tech/", since that is know to the fetcher.
//
// Returns the archive path and the base name.
//
// The ipfs path format is: distribution/version/archiveName
//   - distribution is the name of a distribution, such as "go-ipfs"
//   - version is the version to fetch, such as "v0.8.0-rc2"
//   - archiveName is formatted as name_version_osv-GOARCH.atype, such as
//     "go-ipfs_v0.8.0-rc2_linux-amd64.tar.gz"
//
// This would form the path:
// go-ipfs/v0.8.0/go-ipfs_v0.8.0_linux-amd64.tar.gz
func makeArchivePath(dist, name, ver, atype string) (string, string) {
	arcName := fmt.Sprintf("%s_%s_%s-%s.%s", name, ver, runtime.GOOS, runtime.GOARCH, atype)
	return fmt.Sprintf("%s/%s/%s", dist, ver, arcName), arcName
}
