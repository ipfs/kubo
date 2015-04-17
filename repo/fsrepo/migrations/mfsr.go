package mfsr

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

const VersionFile = "version"

type RepoPath string

func (rp RepoPath) VersionFile() string {
	return path.Join(string(rp), VersionFile)
}

func (rp RepoPath) Version() (string, error) {
	if rp == "" {
		return "", fmt.Errorf("invalid repo path \"%s\"", rp)
	}

	fn := rp.VersionFile()
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		return "", VersionFileNotFound(rp)
	}

	c, err := ioutil.ReadFile(fn)
	if err != nil {
		return "", err
	}

	s := string(c)
	s = strings.TrimSpace(s)
	return s, nil
}

func (rp RepoPath) CheckVersion(version string) error {
	v, err := rp.Version()
	if err != nil {
		return err
	}

	if v != version {
		return fmt.Errorf("versions differ (expected: %s, actual:%s)", version, v)
	}

	return nil
}

func (rp RepoPath) WriteVersion(version string) error {
	fn := rp.VersionFile()
	return ioutil.WriteFile(fn, []byte(version+"\n"), 0644)
}

type VersionFileNotFound string

func (v VersionFileNotFound) Error() string {
	return "no version file in repo at " + string(v)
}
