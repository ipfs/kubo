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

func init() {
	homedir.DisableCache = true
}

// ApiEndpoint reads the api file from the local ipfs install directory and
// returns the address:port read from the file.  If the ipfs directory is not
// specified then the default location is used.
func ApiEndpoint(ipfsDir string) (string, error) {
	ipfsDir, err := CheckIpfsDir(ipfsDir)
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

// IpfsDir returns the path of the ipfs directory.  If dir specified, then
// returns the expanded version dir.  If dir is "", then return the directory
// set by IPFS_PATH, or if IPFS_PATH is not set, then return the default
// location in the home directory.
func IpfsDir(dir string) (string, error) {
	var err error
	if dir != "" {
		dir, err = homedir.Expand(dir)
		if err != nil {
			return "", err
		}
		return dir, nil
	}

	ipfspath := os.Getenv(envIpfsPath)
	if ipfspath != "" {
		dir, err := homedir.Expand(ipfspath)
		if err != nil {
			return "", err
		}
		return dir, nil
	}

	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", errors.New("could not determine IPFS_PATH, home dir not set")
	}

	return path.Join(home, ".ipfs"), nil
}

// CheckIpfsDir gets the ipfs directory and checks that the directory exists.
func CheckIpfsDir(dir string) (string, error) {
	var err error
	dir, err = IpfsDir(dir)
	if err != nil {
		return "", err
	}

	_, err = os.Stat(dir)
	if err != nil {
		return "", err
	}

	return dir, nil
}

// RepoVersion returns the version of the repo in the ipfs directory.  If the
// ipfs directory is not specified then the default location is used.
func RepoVersion(ipfsDir string) (int, error) {
	ipfsDir, err := CheckIpfsDir(ipfsDir)
	if err != nil {
		return 0, err
	}
	return repoVersion(ipfsDir)
}

// WriteRepoVersion writes the specified repo version to the repo located in
// ipfsDir. If ipfsDir is not specified, then the default location is used.
func WriteRepoVersion(ipfsDir string, version int) error {
	ipfsDir, err := IpfsDir(ipfsDir)
	if err != nil {
		return err
	}

	vFilePath := path.Join(ipfsDir, versionFile)
	return ioutil.WriteFile(vFilePath, []byte(fmt.Sprintf("%d\n", version)), 0644)
}

func repoVersion(ipfsDir string) (int, error) {
	c, err := ioutil.ReadFile(path.Join(ipfsDir, versionFile))
	if err != nil {
		return 0, err
	}

	ver, err := strconv.Atoi(strings.TrimSpace(string(c)))
	if err != nil {
		return 0, errors.New("invalid data in repo version file")
	}
	return ver, nil
}
