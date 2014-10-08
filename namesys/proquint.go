package namesys

import (
	"errors"

	proquint "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/bren2010/proquint"
)

type ProquintResolver struct{}

// CanResolve implements Resolver. Checks whether the name is a proquint string.
func (r *ProquintResolver) CanResolve(name string) bool {
	ok, err := proquint.IsProquint(name)
	return err == nil && ok
}

// Resolve implements Resolver. Decodes the proquint string.
func (r *ProquintResolver) Resolve(name string) (string, error) {
	ok := r.CanResolve(name)
	if !ok {
		return "", errors.New("not a valid proquint string")
	}
	return string(proquint.Decode(name)), nil
}
