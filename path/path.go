package path

import (
	"path"
	"strings"

	u "github.com/jbenet/go-ipfs/util"
)

// TODO: debate making this a private struct wrapped in a public interface
// would allow us to control creation, and cache segments.
type Path string

// FromString safely converts a string type to a Path type
func FromString(s string) Path {
	return Path(s)
}

// FromKey safely converts a Key type to a Path type
func FromKey(k u.Key) Path {
	return Path(k.String())
}

func (p Path) Segments() []string {
	cleaned := path.Clean(string(p))
	segments := strings.Split(cleaned, "/")

	// Ignore leading slash
	if len(segments[0]) == 0 {
		segments = segments[1:]
	}

	return segments
}

func (p Path) String() string {
	return string(p)
}
