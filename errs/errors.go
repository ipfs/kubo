package errs

import (
	"errors"
	"fmt"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
)

var ErrCidNotFound = errors.New("CID not found in local blockstore")

// RetrievalError is an error indicating that a Cid could not be
// retrived for some reason
type RetrievalError struct {
	Err error
	Cid *cid.Cid
}

func (e *RetrievalError) Cause() error {
	return e.Err
}

func (e *RetrievalError) Error() string {
	return fmt.Sprintf("could not retrieve %s: %s", e.Cid.String(), e.Err.Error())
}
