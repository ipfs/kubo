package data

import (
	"fmt"
	"os"
)

var Debug bool
var NotImplementedError = fmt.Errorf("Error: not implemented yet.")

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
