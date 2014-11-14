// package util implements various utility functions used within ipfs
// that do not currently have a better place to live.
package util

import (
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/mitchellh/go-homedir"
)

// Debug is a global flag for debugging.
var Debug bool

// ErrNotImplemented signifies a function has not been implemented yet.
var ErrNotImplemented = errors.New("Error: not implemented yet.")

// ErrTimeout implies that a timeout has been triggered
var ErrTimeout = errors.New("Error: Call timed out.")

// ErrSeErrSearchIncomplete implies that a search type operation didnt
// find the expected node, but did find 'a' node.
var ErrSearchIncomplete = errors.New("Error: Search Incomplete.")

// ErrNotFound is returned when a search fails to find anything
var ErrNotFound = ds.ErrNotFound

// ErrNoSuchLogger is returned when the util pkg is asked for a non existant logger
var ErrNoSuchLogger = errors.New("Error: No such logger")

// TildeExpansion expands a filename, which may begin with a tilde.
func TildeExpansion(filename string) (string, error) {
	return homedir.Expand(filename)
}

// ErrCast is returned when a cast fails AND the program should not panic.
func ErrCast() error {
	debug.PrintStack()
	return errCast
}

var errCast = errors.New("cast error")

// ExpandPathnames takes a set of paths and turns them into absolute paths
func ExpandPathnames(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		abspath, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		out = append(out, abspath)
	}
	return out, nil
}

// byteChanReader wraps a byte chan in a reader
type byteChanReader struct {
	in  chan []byte
	buf []byte
}

func NewByteChanReader(in chan []byte) io.Reader {
	return &byteChanReader{in: in}
}

func (bcr *byteChanReader) Read(output []byte) (int, error) {
	remain := output
	remainLen := len(output)
	outputLen := 0
	more := false
	next := bcr.buf

	for {
		n := copy(remain, next)
		remainLen -= n
		outputLen += n
		if remainLen == 0 {
			bcr.buf = next[n:]
			return outputLen, nil
		}

		remain = remain[n:]
		next, more = <-bcr.in
		if !more {
			return outputLen, io.EOF
		}
	}
}

type randGen struct {
	rand.Rand
}

func NewTimeSeededRand() io.Reader {
	src := rand.NewSource(time.Now().UnixNano())
	return &randGen{
		Rand: *rand.New(src),
	}
}

func (r *randGen) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = byte(r.Rand.Intn(255))
	}
	return len(p), nil
}

// GetenvBool is the way to check an env var as a boolean
func GetenvBool(name string) bool {
	v := strings.ToLower(os.Getenv(name))
	return v == "true" || v == "t" || v == "1"
}
