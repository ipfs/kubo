package errs

import (
	"errors"
)

// Unwrap recursively unwrapped an error to an error type that can be
// tested for quality.  Noop for now.
func Unwrap(err error) error {
	return err
}

var ErrCidNotFound = errors.New("CID not found in local blockstore")
