package namesys

import (
	"errors"

	proquint "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/bren2010/proquint"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	path "github.com/ipfs/go-ipfs/path"
)

type ProquintResolver struct{}

// CanResolve implements Resolver. Checks whether the name is a proquint string.
func (r *ProquintResolver) CanResolve(name string) bool {
	ok, err := proquint.IsProquint(name)
	return err == nil && ok
}

// Resolve implements Resolver. Decodes the proquint string.
func (r *ProquintResolver) Resolve(ctx context.Context, name string) (path.Path, error) {
	ok := r.CanResolve(name)
	if !ok {
		return "", errors.New("not a valid proquint string")
	}
	return path.FromString(string(proquint.Decode(name))), nil
}
