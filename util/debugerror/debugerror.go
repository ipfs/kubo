// package debugerror provides a way to augment errors with additional
// information to allow for easier debugging.
package debugerror

import (
	"errors"
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/facebookgo/stackerr"
	"github.com/jbenet/go-ipfs/util"
)

func Errorf(format string, a ...interface{}) error {
	return Wrap(fmt.Errorf(format, a...))
}

// New returns an error that contains a stack trace (in debug mode)
func New(s string) error {
	if util.Debug {
		return stackerr.New(s)
	}
	return errors.New(s)
}

func Wrap(err error) error {
	if util.Debug {
		return stackerr.Wrap(err)
	}
	return err
}
