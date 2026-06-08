package ipfs

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/ipfs/kubo/core/commands/cmdutils"
)

// CurrentCommit is the current git commit, this is set as a ldflag in the Makefile.
var CurrentCommit string

// taggedRelease is set via ldflag when building from a version-tagged commit
// with a clean tree. When set, the commit hash is omitted from the libp2p
// identify agent version and the HTTP user agent, since the version number
// already identifies the exact source.
var taggedRelease string

// buildOrigin is the Makefile-injected `host/org/repo` form of
// `git remote get-url origin`. ImplicitAgentSuffix turns a non-upstream
// value into the Version.AgentSuffix default so fork builds self-identify.
var buildOrigin string

// upstreamModulePath is the canonical upstream module path. Builds whose
// origin matches it contribute no implicit suffix.
const upstreamModulePath = "github.com/ipfs/kubo"

// CurrentVersionNumber is the current application's version literal.
const CurrentVersionNumber = "0.42.0"

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

// ImplicitAgentSuffix returns a Version.AgentSuffix default derived from
// the build origin. It prefers the Makefile-injected URL (covers forks
// that keep the upstream `module` line) and falls back to
// debug.ReadBuildInfo's main module path (covers `go install` and forks
// that renamed their module). Returns "" for upstream builds.
func ImplicitAgentSuffix() string {
	if s := suffixFromForkPath(buildOrigin); s != "" {
		return s
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		return suffixFromForkPath(bi.Main.Path)
	}
	return ""
}

// knownForges lists public git hosts whose hostname is dropped from the
// implicit suffix; other hosts are kept so the origin stays identifiable.
var knownForges = map[string]struct{}{
	"github.com":    {},
	"gitlab.com":    {},
	"codeberg.org":  {},
	"bitbucket.org": {},
}

// suffixFromForkPath turns a normalized `host/org/repo` into the implicit
// Version.AgentSuffix. Returns "" for upstream and empty inputs.
func suffixFromForkPath(p string) string {
	p = strings.Trim(p, "/")
	if p == "" || p == upstreamModulePath {
		return ""
	}
	parts := strings.Split(p, "/")
	// Only normalize canonical `host/org/repo`; shorter inputs pass through
	// so operators can still identify them.
	if len(parts) < 3 {
		return p
	}
	if _, ok := knownForges[parts[0]]; ok {
		parts = parts[1:]
	}
	if parts[len(parts)-1] == "kubo" {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, "/")
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
