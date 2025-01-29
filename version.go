package ipfs

import (
	"fmt"
	"runtime"

	"github.com/ipfs/kubo/repo/fsrepo"
)

// CurrentCommit is the current git commit, this is set as a ldflag in the Makefile.
var CurrentCommit string

// CurrentVersionNumber is the current application's version literal.
const CurrentVersionNumber = "0.34.0-dev"

const ApiVersion = "/kubo/" + CurrentVersionNumber + "/" //nolint

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
	return userAgent
}

var userAgentSuffix string

func SetUserAgentSuffix(suffix string) {
	userAgentSuffix = suffix
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
		System:  runtime.GOARCH + "/" + runtime.GOOS, // TODO: Precise version here
		Golang:  runtime.Version(),
	}
}
