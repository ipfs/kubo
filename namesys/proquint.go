package namesys

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proquint "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/bren2010/proquint"
	u "github.com/jbenet/go-ipfs/util"
)

type ProquintResolver struct{}

// CanResolve implements Resolver. Checks whether the name is a proquint string.
func (r *ProquintResolver) CanResolve(name string) bool {
	ok, err := proquint.IsProquint(name)
	return err == nil && ok
}

// Resolve implements Resolver. Decodes the proquint string.
func (r *ProquintResolver) Resolve(ctx context.Context, name string) (u.Key, error) {
	ok := r.CanResolve(name)
	if !ok {
		return "", errors.New("not a valid proquint string")
	}
	return u.Key(proquint.Decode(name)), nil
}
