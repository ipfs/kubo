package version

import semver "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"

var currentVersion = &semver.Version{
	Major: 0,
	Minor: 1,
	Patch: 0,
}

// Current returns the current protocol version as a protobuf message
func Current() *SemVer {
	return toPBSemVer(currentVersion)
}

// Compatible checks wether two versions are compatible
func Compatible(a, b semver.Version) bool {
	return !a.LessThan(b)
}

// toPBSemVar converts a coreos/semver to our protobuf SemVer
func toPBSemVer(in *semver.Version) (out *SemVer) {

	return &SemVer{
		Major: &in.Major,
		Minor: &in.Minor,
		Patch: &in.Patch,
	}
}

// Convert our protobuf SemVer to a coreos/semver
func (in SemVer) Convert() semver.Version {
	return semver.Version{
		Major: *in.Major,
		Minor: *in.Minor,
		Patch: *in.Patch,
	}
}
