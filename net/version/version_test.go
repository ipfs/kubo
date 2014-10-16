package version

import (
	"testing"

	semver "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
)

func TestCompatible(t *testing.T) {
	tcases := []struct {
		a, b     semver.Version
		expected bool
	}{
		{semver.Version{Major: 0}, semver.Version{Major: 0}, true},
		{semver.Version{Major: 1}, semver.Version{Major: 0}, true},
		{semver.Version{Major: 1}, semver.Version{Major: 1}, true},
		{semver.Version{Major: 0}, semver.Version{Major: 1}, false},
	}

	for i, tcase := range tcases {
		if Compatible(tcase.a, tcase.b) != tcase.expected {
			t.Fatalf("case[%d] failed", i)
		}
	}
}
