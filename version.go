package ipfs

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// CurrentCommit is the current git commit, this is set as a ldflag in the Makefile.
var CurrentCommit string

// CurrentVersionNumber is the current application's version literal.
const CurrentVersionNumber = "0.38.0-dev"

const ApiVersion = "/kubo/" + CurrentVersionNumber + "/" //nolint

// RepoVersion is the version number that we are currently expecting to see.
const RepoVersion = 17

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

// nonPrintableASCII matches characters outside the printable ASCII range (space through tilde)
var nonPrintableASCII = regexp.MustCompile(`[^\x20-\x7E]+`)

func SetUserAgentSuffix(suffix string) {
	userAgentSuffix = TrimVersion(suffix)
}

// TrimVersion sanitizes version strings to contain only printable ASCII
// limited to maxVersionLen characters. Non-printable characters are replaced with underscores.
func TrimVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}

	// Replace runs of non-printable ASCII with single underscore
	sanitized := nonPrintableASCII.ReplaceAllString(version, "_")

	// Truncate to max length (safe since we have ASCII-only)
	if len(sanitized) > maxVersionLen {
		sanitized = sanitized[:maxVersionLen]
	}

	return sanitized
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
