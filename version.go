package ipfs

import (
	"fmt"
	"runtime"

	"github.com/ipfs/kubo/core/commands/cmdutils"
)

// CurrentCommit is the current git commit, this is set as a ldflag in the Makefile.
var CurrentCommit string

// taggedRelease is set via ldflag when building from a version-tagged commit
// with a clean tree. When set, the commit hash is omitted from the libp2p
// identify agent version and the HTTP user agent, since the version number
// already identifies the exact source.
var taggedRelease string

// CurrentVersionNumber is the current application's version literal.
const CurrentVersionNumber = "0.41.0-dev"

const ApiVersion = "/kubo/" + CurrentVersionNumber + "/" //nolint

// RepoVersion is the version number that we are currently expecting to see.
const RepoVersion = 18

// GetUserAgentVersion is the libp2p user agent used by go-ipfs.
func GetUserAgentVersion() string {
	// For tagged release builds with a clean tree, the commit hash is
	// redundant since the version number identifies the exact source.
	commit := CurrentCommit
	if taggedRelease != "" {
		commit = ""
	}

	userAgent := "kubo/" + CurrentVersionNumber
	if commit != "" {
		userAgent += "/" + commit
	}
	if userAgentSuffix != "" {
		userAgent += "/" + userAgentSuffix
	}
	return cmdutils.CleanAndTrim(userAgent)
}

var userAgentSuffix string

func SetUserAgentSuffix(suffix string) {
	userAgentSuffix = cmdutils.CleanAndTrim(suffix)
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
		Repo:    fmt.Sprint(RepoVersion),
		System:  runtime.GOARCH + "/" + runtime.GOOS, // TODO: Precise version here
		Golang:  runtime.Version(),
	}
}
