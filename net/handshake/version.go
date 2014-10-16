package handshake

// currentVersion holds the current protocol version for a client running this code
var currentVersion = NewSemVer(0, 0, 1)

// Current returns the current protocol version as a protobuf message
func Current() *SemVer {
	return currentVersion
}

// Compatible checks wether two versions are compatible
func Compatible(a, b *SemVer) bool {
	return *a.Major == *b.Major // protobuf fields are pointers
}

// NewSemVer constructs a new protobuf SemVer
func NewSemVer(major, minor, patch int64) *SemVer {
	s := new(SemVer)
	s.Major = &major
	s.Minor = &minor
	s.Patch = &patch
	return s
}
