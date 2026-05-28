package ipfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuffixFromForkPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{name: "empty", path: "", expected: ""},
		{name: "upstream", path: "github.com/ipfs/kubo", expected: ""},
		{name: "github fork", path: "github.com/myorg/kubo", expected: "myorg"},
		{name: "gitlab fork", path: "gitlab.com/myorg/kubo", expected: "myorg"},
		{name: "codeberg fork", path: "codeberg.org/myorg/kubo", expected: "myorg"},
		{name: "bitbucket fork", path: "bitbucket.org/myorg/kubo", expected: "myorg"},
		{name: "github renamed repo", path: "github.com/myorg/kubo-experimental", expected: "myorg/kubo-experimental"},
		{name: "unknown host canonical repo", path: "git.example.com/team/kubo", expected: "git.example.com/team"},
		{name: "unknown host renamed repo", path: "git.example.com/team/kubo-fork", expected: "git.example.com/team/kubo-fork"},
		{name: "unknown host nested path", path: "git.example.com/group/sub/kubo", expected: "git.example.com/group/sub"},
		{name: "trailing slash", path: "github.com/myorg/kubo/", expected: "myorg"},
		{name: "leading slash", path: "/github.com/myorg/kubo", expected: "myorg"},
		{name: "single segment", path: "kubo", expected: "kubo"},
		{name: "two segment fork on known host", path: "github.com/kubo", expected: "github.com/kubo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, suffixFromForkPath(tt.path))
		})
	}
}

func TestImplicitAgentSuffix_PrefersBuildOrigin(t *testing.T) {
	orig := buildOrigin
	t.Cleanup(func() { buildOrigin = orig })

	buildOrigin = "github.com/myorg/kubo"
	assert.Equal(t, "myorg", ImplicitAgentSuffix())

	// Falls through to BuildInfo when origin matches upstream or is empty;
	// BuildInfo.Main.Path is "github.com/ipfs/kubo" during `go test` of this
	// package, so the implicit suffix is empty.
	buildOrigin = ""
	assert.Equal(t, "", ImplicitAgentSuffix())

	buildOrigin = upstreamModulePath
	assert.Equal(t, "", ImplicitAgentSuffix())
}

// TestGetUserAgentVersion verifies the user agent string used in libp2p
// identify and HTTP requests. Tagged release builds (where the commit matches
// the tag) skip the commit hash from the agent version, since the version
// number already identifies the exact source.
func TestGetUserAgentVersion(t *testing.T) {
	origCommit := CurrentCommit
	origTagged := taggedRelease
	origSuffix := userAgentSuffix
	t.Cleanup(func() {
		CurrentCommit = origCommit
		taggedRelease = origTagged
		userAgentSuffix = origSuffix
	})

	tests := []struct {
		name     string
		commit   string
		tagged   string
		suffix   string
		expected string
	}{
		// dev builds without ldflags
		{
			name:     "no commit, no suffix",
			expected: "kubo/" + CurrentVersionNumber,
		},
		// dev builds with commit set via ldflags
		{
			name:     "with commit",
			commit:   "abc1234",
			expected: "kubo/" + CurrentVersionNumber + "/abc1234",
		},
		{
			name:     "with suffix, no commit",
			suffix:   "test-suffix",
			expected: "kubo/" + CurrentVersionNumber + "/test-suffix",
		},
		{
			name:     "with commit and suffix",
			commit:   "abc1234",
			suffix:   "test-suffix",
			expected: "kubo/" + CurrentVersionNumber + "/abc1234/test-suffix",
		},
		// tagged release builds: commit is redundant because the version
		// number already maps to an exact git tag, so it is omitted to
		// save bytes in identify and HTTP user-agent headers.
		{
			name:     "tagged release ignores commit",
			commit:   "abc1234",
			tagged:   "1",
			expected: "kubo/" + CurrentVersionNumber,
		},
		{
			name:     "tagged release with suffix ignores commit",
			commit:   "abc1234",
			tagged:   "1",
			suffix:   "test-suffix",
			expected: "kubo/" + CurrentVersionNumber + "/test-suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CurrentCommit = tt.commit
			taggedRelease = tt.tagged
			SetUserAgentSuffix(tt.suffix)

			assert.Equal(t, tt.expected, GetUserAgentVersion())
		})
	}
}
