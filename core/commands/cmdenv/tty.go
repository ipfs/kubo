package cmdenv

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsTerminal reports whether f is connected to a terminal,
// including MSYS/Cygwin-style terminals on Windows.
func IsTerminal(f *os.File) bool {
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
