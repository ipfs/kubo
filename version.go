package ipfs

import (
	"fmt"
	"regexp"
	"runtime"

	"github.com/ipfs/kubo/repo/fsrepo"
)

// CurrentCommit is the current git commit, this is set as a ldflag in the Makefile
var CurrentCommit string

// CurrentVersionNumber is the current application's version literal
const CurrentVersionNumber = "0.18.0-dev"

const ApiVersion = "/kubo/" + CurrentVersionNumber + "/" //nolint

const maxVersionLen = 64

// GetUserAgentVersion is the libp2p user agent used by go-ipfs.
//
// Note: This will end in `/` when no commit is available. This is expected.
func GetUserAgentVersion() string {
	userAgent := "kubo/" + CurrentVersionNumber + "/" + CurrentCommit
	if userAgentSuffix != "" {
		if CurrentCommit != "" {
			userAgent += "/"
		}
		userAgent += userAgentSuffix
	}
	return TrimVersion(userAgent)
}

var userAgentSuffix string
var onlyASCII = regexp.MustCompile("[[:^ascii:]]")

func SetUserAgentSuffix(suffix string) {
	userAgentSuffix = TrimVersion(suffix)
}

func TrimVersion(version string) string {
	ascii := onlyASCII.ReplaceAllLiteralString(version, "")
	chars := 0
	for i := range ascii {
		if chars >= maxVersionLen {
			ascii = ascii[:i]
			break
		}
		chars++
	}
	return ascii
}

type VersionInfo struct {
	Version string
	Commit  string
	Repo    string
	System  string
	Golang  string
}

func GetVersionInfo() *VersionInfo {
	return &VersionInfo{
		Version: CurrentVersionNumber,
		Commit:  CurrentCommit,
		Repo:    fmt.Sprint(fsrepo.RepoVersion),
		System:  runtime.GOARCH + "/" + runtime.GOOS, //TODO: Precise version here
		Golang:  runtime.Version(),
	}
}
