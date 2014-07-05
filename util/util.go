package util

import (
	"fmt"
	mh "github.com/jbenet/go-multihash"
	"os"
)

var Debug bool
var NotImplementedError = fmt.Errorf("Error: not implemented yet.")

// a Key for maps. It's a string (rep of a multihash).
type Key string

// global hash function. uses multihash SHA2_256, 256 bits
func Hash(data []byte) (mh.Multihash, error) {
	return mh.Sum(data, mh.SHA2_256, -1)
}

// Shorthand printing functions.
func PErr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func POut(format string, a ...interface{}) {
	fmt.Fprintf(os.Stdout, format, a...)
}

func DErr(format string, a ...interface{}) {
	if Debug {
		PErr(format, a...)
	}
}

func DOut(format string, a ...interface{}) {
	if Debug {
		POut(format, a...)
	}
}
