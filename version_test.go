package ipfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
