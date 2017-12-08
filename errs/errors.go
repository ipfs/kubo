package errs

import (
	"errors"
	"fmt"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
)

var ErrCidNotFound = errors.New("CID not found in local blockstore")

// Unwrap recursively unwrapped an error to an error type that can be
// tested for quality
func Unwrap(err error) error {
	switch e := err.(type) {
	case Wrapped:
		return Unwrap(e.Unwrap())
	default:
		return err
	}
}

// Wrapped represent an error wrapped with additional information,
// such as a Cid
type Wrapped interface {
	Unwrap() error
}

// RetrievalError is an error indicating that a Cid could not be
// retrived for some reason
type RetrievalError struct {
	Err error
	Cid *cid.Cid
}

func (e *RetrievalError) Unwrap() error {
	return e.Err
}

func (e *RetrievalError) Error() string {
	return fmt.Sprintf("could not retrieve %s: %s", e.Cid.String(), e.Err.Error())
}
