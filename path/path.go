package path

import (
	"path"
	"strings"
)

// TODO: debate making this a private struct wrapped in a public interface
// would allow us to control creation, and cache segments.
type Path string

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
