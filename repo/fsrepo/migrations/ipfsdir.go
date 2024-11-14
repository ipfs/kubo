package migrations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ipfs/kubo/misc/fsutil"
)

const (
	envIpfsPath = "IPFS_PATH"
	defIpfsDir  = ".ipfs"
	versionFile = "version"
)

// IpfsDir returns the path of the ipfs directory.  If dir specified, then
// returns the expanded version dir.  If dir is "", then return the directory
// set by IPFS_PATH, or if IPFS_PATH is not set, then return the default
// location in the home directory.
func IpfsDir(dir string) (string, error) {
	var err error
	if dir == "" {
		dir = os.Getenv(envIpfsPath)
	}
	if dir != "" {
		dir, err = fsutil.ExpandHome(dir)
		if err != nil {
			return "", err
		}
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", errors.New("could not determine IPFS_PATH, home dir not set")
	}

	return filepath.Join(home, defIpfsDir), nil
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

	vFilePath := filepath.Join(ipfsDir, versionFile)
	return os.WriteFile(vFilePath, []byte(fmt.Sprintf("%d\n", version)), 0o644)
}

func repoVersion(ipfsDir string) (int, error) {
	c, err := os.ReadFile(filepath.Join(ipfsDir, versionFile))
	if err != nil {
		return 0, err
	}

	ver, err := strconv.Atoi(strings.TrimSpace(string(c)))
	if err != nil {
		return 0, errors.New("invalid data in repo version file")
	}
	return ver, nil
}
