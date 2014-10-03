package util

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	logging "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/op/go-logging"
)

// LogFormat is the format used for our logger.
var LogFormat = "%{color}%{time:01-02 15:04:05.9999} %{shortfile} %{level}: %{color:reset}%{message}"

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

// Key is a string representation of multihash for use with maps.
type Key string

// Pretty returns Key in a b58 encoded string
func (k Key) Pretty() string {
	return b58.Encode([]byte(k))
}

// DsKey returns a Datastore key
func (k Key) DsKey() ds.Key {
	return ds.NewKey(k.Pretty())
}

// KeyFromDsKey returns a Datastore key
func KeyFromDsKey(dsk ds.Key) Key {
	l := dsk.List()
	enc := l[len(l)-1]
	return Key(b58.Decode(enc))
}

// Hash is the global IPFS hash function. uses multihash SHA2_256, 256 bits
func Hash(data []byte) (mh.Multihash, error) {
	return mh.Sum(data, mh.SHA2_256, -1)
}

// IsValidHash checks whether a given hash is valid (b58 decodable, len > 0)
func IsValidHash(s string) bool {
	out := b58.Decode(s)
	if out == nil || len(out) == 0 {
		return false
	}
	return true
}

// TildeExpansion expands a filename, which may begin with a tilde.
func TildeExpansion(filename string) (string, error) {
	if strings.HasPrefix(filename, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}

		dir := usr.HomeDir + "/"
		filename = strings.Replace(filename, "~/", dir, 1)
	}
	return filename, nil
}

// PErr is a shorthand printing function to output to Stderr.
func PErr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

// POut is a shorthand printing function to output to Stdout.
func POut(format string, a ...interface{}) {
	fmt.Fprintf(os.Stdout, format, a...)
}

// DErr is a shorthand debug printing function to output to Stderr.
// Will only print if Debug is true.
func DErr(format string, a ...interface{}) {
	if Debug {
		PErr(format, a...)
	}
}

// DOut is a shorthand debug printing function to output to Stdout.
// Will only print if Debug is true.
func DOut(format string, a ...interface{}) {
	if Debug {
		POut(format, a...)
	}
}

// SetupLogging will initialize the logger backend and set the flags.
func SetupLogging() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(backend)
	/*
		if Debug {
			logging.SetLevel(logging.DEBUG, "")
		} else {
			logging.SetLevel(logging.ERROR, "")
		}
	*/
	logging.SetLevel(logging.ERROR, "merkledag")
	logging.SetLevel(logging.ERROR, "blockservice")
	logging.SetFormatter(logging.MustStringFormatter(LogFormat))
}

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
