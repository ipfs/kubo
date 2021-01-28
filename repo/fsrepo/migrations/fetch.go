package migrations

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

const (
	envIpfsDistPath = "IPFS_DIST_PATH"

	// Distribution
	gatewayURL = "https://ipfs.io"
	ipfsDist   = "/ipns/dist.ipfs.io"

	// Maximum download size
	fetchSizeLimit = 1024 * 1024 * 512
)

type limitReadCloser struct {
	io.Reader
	io.Closer
}

var ipfsDistPath string

func init() {
	SetIpfsDistPath("")
}

// SetIpfsDistPath sets the ipfs path to the distribution site.  If an empty
// string is given, then the path is set using the IPFS_DIST_PATH environ
// veriable, or the default value if that is not defined.
func SetIpfsDistPath(distPath string) {
	if distPath != "" {
		ipfsDistPath = distPath
		return
	}

	if dist := os.Getenv(envIpfsDistPath); dist != "" {
		ipfsDistPath = dist
	} else {
		ipfsDistPath = ipfsDist
	}
}

// FetchBinary downloads an archive from the distribution site and unpacks it.
//
// The base name of the archive file, inside the distribution directory on
// distribution site, may differ from the distribution name.  If it does, then
// specify arcName.
//
// The base name of the binary inside the archive may differ from the base
// archive name.  If it does, then specify binName.  For example, the following
// is needed because the archive "go-ipfs_v0.7.0_linux-amd64.tar.gz" contains a
// binary named "ipfs"
//
//     FetchBinary(ctx, "go-ipfs", "v0.7.0", "go-ipfs", "ipfs", tmpDir)
//
// If out is a directory, then the binary is written to that directory with the
// same name it has inside the archive.  Otherwise, the binary file is written
// to the file named by out.
func FetchBinary(ctx context.Context, dist, ver, arcName, binName, out string) (string, error) {
	// If archive base name not specified, then it is same as dist.
	if arcName == "" {
		arcName = dist
	}
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
		out = path.Join(out, binName)
	}

	// Create temp directory to store download
	tmpDir, err := ioutil.TempDir("", arcName)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	atype := "tar.gz"
	if runtime.GOOS == "windows" {
		atype = "zip"
	}

	arcName = makeArchiveName(arcName, ver, atype)
	arcIpfsPath := makeIpfsPath(dist, ver, arcName)

	// Create a file to write the archive data to
	arcPath := path.Join(tmpDir, arcName)
	arcFile, err := os.Create(arcPath)
	if err != nil {
		return "", err
	}
	defer arcFile.Close()

	// Open connection to download archive from ipfs path
	rc, err := fetch(ctx, arcIpfsPath)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	// Write download data
	_, err = io.Copy(arcFile, rc)
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
	err = os.Chmod(out, 0755)
	if err != nil {
		return "", err
	}

	return out, nil
}

// fetch attempts to fetch the file at the given ipfs path, first using the
// local ipfs api if available, then using http.  Returns io.ReadCloser on
// success, which caller must close.
func fetch(ctx context.Context, ipfsPath string) (io.ReadCloser, error) {
	// Try fetching via ipfs daemon
	rc, err := ipfsFetch(ctx, ipfsPath)
	if err == nil {
		// Transferred using local ipfs daemon
		return rc, nil
	}
	// Try fetching via HTTP
	return httpFetch(ctx, gatewayURL+ipfsPath)
}

// ipfsFetch attempts to fetch the file at the given ipfs path using the local
// ipfs api.  Returns io.ReadCloser on success, which caller must close.
func ipfsFetch(ctx context.Context, ipfsPath string) (io.ReadCloser, error) {
	sh, _, err := ApiShell("")
	if err != nil {
		return nil, err
	}

	resp, err := sh.Request("cat", ipfsPath).Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	return newLimitReadCloser(resp.Output, fetchSizeLimit), nil
}

// httpFetch attempts to fetch the file at the given URL.  Returns
// io.ReadCloser on success, which caller must close.
func httpFetch(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest error: %s", err)
	}

	req.Header.Set("User-Agent", "go-ipfs")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do error: %s", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		mes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading error body: %s", err)
		}
		return nil, fmt.Errorf("GET %s error: %s: %s", url, resp.Status, string(mes))
	}

	return newLimitReadCloser(resp.Body, fetchSizeLimit), nil
}

func newLimitReadCloser(rc io.ReadCloser, limit int64) io.ReadCloser {
	return limitReadCloser{
		Reader: io.LimitReader(rc, limit),
		Closer: rc,
	}
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

// makeArchiveName composes the name of a migration binary archive.
//
// The archive name is in the format: name_version_osv-GOARCH.atype
// Example: ipfs-10-to-11_v1.8.0_darwin-amd64.tar.gz
func makeArchiveName(name, ver, atype string) string {
	return fmt.Sprintf("%s_%s_%s-%s.%s", name, ver, runtime.GOOS, runtime.GOARCH, atype)
}

// makeIpfsPath composes the name ipfs path location to download a migration
// binary from the distribution site.
//
// The ipfs path format: distBaseCID/rootdir/version/name/archive
func makeIpfsPath(dist, ver, arcName string) string {
	return fmt.Sprintf("%s/%s/%s/%s", ipfsDistPath, dist, ver, arcName)
}
