package updates

import (
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	"github.com/jbenet/go-ipfs/config"
)

// TestParseVersion just makes sure that we dont commit a bad version number
func TestParseVersion(t *testing.T) {
	_, err := parseVersion()
	if err != nil {
		t.Fatal(err)
	}
}

func TestShouldAutoUpdate(t *testing.T) {
	tests := []struct {
		setting, currV, newV string
		should               bool
	}{
		{config.UpdateNever, "0.0.1", "1.0.0", false},
		{config.UpdateNever, "0.0.1", "0.1.0", false},
		{config.UpdateNever, "0.0.1", "0.0.1", false},
		{config.UpdateNever, "0.0.1", "0.0.2", false},

		{config.UpdatePatch, "0.0.1", "1.0.0", false},
		{config.UpdatePatch, "0.0.1", "0.1.0", false},
		{config.UpdatePatch, "0.0.1", "0.0.1", false},
		{config.UpdatePatch, "0.0.2", "0.0.1", false},
		{config.UpdatePatch, "0.0.1", "0.0.2", true},

		{config.UpdateMinor, "0.1.1", "1.0.0", false},
		{config.UpdateMinor, "0.1.1", "0.2.0", true},
		{config.UpdateMinor, "0.1.1", "0.1.2", true},
		{config.UpdateMinor, "0.2.1", "0.1.9", false},
		{config.UpdateMinor, "0.1.2", "0.1.1", false},

		{config.UpdateMajor, "1.0.0", "2.0.0", true},
		{config.UpdateMajor, "1.0.0", "1.1.0", true},
		{config.UpdateMajor, "1.0.0", "1.0.1", true},
		{config.UpdateMajor, "2.0.0", "1.0.0", false}, // don't downgrade
		{config.UpdateMajor, "2.5.0", "2.4.0", false},
		{config.UpdateMajor, "2.0.2", "2.0.1", false},
	}

	for i, tc := range tests {
		var err error
		currentVersion, err = semver.NewVersion(tc.currV)
		if err != nil {
			t.Fatalf("Could not parse test version: %v", err)
		}

		if tc.should != ShouldAutoUpdate(tc.setting, tc.newV) {
			t.Fatalf("#%d failed for %+v", i, tc)
		}
	}
}
