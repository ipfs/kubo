package migrations

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	api "github.com/ipfs/go-ipfs-api"
	"github.com/mitchellh/go-homedir"
)

const (
	envIpfsPath = "IPFS_PATH"
	versionFile = "version"

	// Local IPFS API
	apiFile        = "api"
	shellUpTimeout = 2 * time.Second
)

var (
	disableDirCache bool
	ipfsDirCache    string
	ipfsDirCacheKey string
)

// ApiEndpoint reads the api file from the local ipfs install directory and
// returns the address:port read from the file.  If the ipfs directory is not
// specified then the default location is used.
func ApiEndpoint(ipfsDir string) (string, error) {
	ipfsDir, err := checkIpfsDir(ipfsDir)
	if err != nil {
		return "", err
	}
	apiPath := path.Join(ipfsDir, apiFile)

	apiData, err := ioutil.ReadFile(apiPath)
	if err != nil {
		return "", err
	}

	val := strings.TrimSpace(string(apiData))
	parts := strings.Split(val, "/")
	if len(parts) != 5 {
		return "", fmt.Errorf("incorrectly formatted api string: %q", val)
	}

	return parts[2] + ":" + parts[4], nil
}

// ApiShell creates a new ipfs api shell and checks that it is up.  If the shell
// is available, then the shell and ipfs version are returned.
func ApiShell(ipfsDir string) (*api.Shell, string, error) {
	apiEp, err := ApiEndpoint("")
	if err != nil {
		return nil, "", err
	}
	sh := api.NewShell(apiEp)
	sh.SetTimeout(shellUpTimeout)
	ver, _, err := sh.Version()
	if err != nil {
		return nil, "", errors.New("ipfs api shell not up")
	}
	sh.SetTimeout(0)
	return sh, ver, nil
}

// Returns the path of the default ipfs directory.
func IpfsDir() (string, error) {
	return checkIpfsDir("")
}

// RepoVersion returns the version of the repo in the ipfs directory.  If the
// ipfs directory is not specified then the default location is used.
func RepoVersion(ipfsDir string) (int, error) {
	ipfsDir, err := checkIpfsDir(ipfsDir)
	if err != nil {
		return 0, err
	}
	return repoVersion(ipfsDir)
}

// WriteRepoVersion writes the specified repo version to the repo located in
// ipfsDir. If ipfsDir is not specified, then the default location is used.
func WriteRepoVersion(ipfsDir string, version int) error {
	ipfsDir, err := checkIpfsDir(ipfsDir)
	if err != nil {
		return err
	}

	vFilePath := path.Join(ipfsDir, versionFile)
	return ioutil.WriteFile(vFilePath, []byte(fmt.Sprintf("%d\n", version)), 0644)
}

// CacheIpfsDir enables or disables caching the location of the ipfs directory.
// Enabled by default, this avoids subsequent search for and check of the same
// ipfs directory.  Disabling the cache may be useful if the location of the
// ipfs directory is expected to change.
func CacheIpfsDir(enable bool) {
	if !enable {
		disableDirCache = true
		ipfsDirCache = ""
		ipfsDirCacheKey = ""
		homedir.DisableCache = true
		homedir.Reset()
	} else {
		homedir.DisableCache = false
		disableDirCache = false
	}
}

func repoVersion(ipfsDir string) (int, error) {
	c, err := ioutil.ReadFile(path.Join(ipfsDir, versionFile))
	if err != nil {
		if os.IsNotExist(err) {
			// IPFS directory exists without version file, so version 0
			return 0, nil
		}
		return 0, fmt.Errorf("cannot read repo version file: %s", err)
	}

	ver, err := strconv.Atoi(strings.TrimSpace(string(c)))
	if err != nil {
		return 0, errors.New("invalid data in repo version file")
	}
	return ver, nil
}

func checkIpfsDir(dir string) (string, error) {
	if dir == ipfsDirCacheKey && ipfsDirCache != "" {
		return ipfsDirCache, nil
	}

	var (
		err   error
		found string
	)
	if dir == "" {
		found, err = findIpfsDir()
		if err != nil {
			return "", fmt.Errorf("could not find ipfs directory: %s", err)
		}
	} else {
		found, err = homedir.Expand(dir)
		if err != nil {
			return "", err
		}

		_, err = os.Stat(found)
		if err != nil {
			return "", err
		}
	}

	if !disableDirCache {
		ipfsDirCacheKey = dir
		ipfsDirCache = found
	}

	return found, nil
}

func findIpfsDir() (string, error) {
	ipfspath := os.Getenv(envIpfsPath)
	if ipfspath != "" {
		expandedPath, err := homedir.Expand(ipfspath)
		if err != nil {
			return "", err
		}
		return expandedPath, nil
	}

	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", errors.New("could not determine IPFS_PATH, home dir not set")
	}

	for _, dir := range []string{".go-ipfs", ".ipfs"} {
		defaultDir := path.Join(home, dir)
		_, err = os.Stat(defaultDir)
		if err == nil {
			return defaultDir, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}

	return "", err
}
