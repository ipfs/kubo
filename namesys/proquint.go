package namesys

import (
	"errors"

	proquint "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/bren2010/proquint"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	path "github.com/ipfs/go-ipfs/path"
	infd "github.com/ipfs/go-ipfs/util/infduration"
)

type ProquintResolver struct{}

// Resolve implements Resolver.
func (r *ProquintResolver) Resolve(ctx context.Context, name string) (path.Path, error) {
	p, _, err := r.ResolveWithTTL(ctx, name)
	return p, err
}

// ResolveN implements Resolver.
func (r *ProquintResolver) ResolveN(ctx context.Context, name string, depth int) (path.Path, error) {
	p, _, err := r.ResolveNWithTTL(ctx, name, depth)
	return p, err
}

// ResolveWithTTL implements Resolver.
func (r *ProquintResolver) ResolveWithTTL(ctx context.Context, name string) (path.Path, infd.Duration, error) {
	return r.ResolveNWithTTL(ctx, name, DefaultDepthLimit)
}

// ResolveNWithTTL implements Resolver.
func (r *ProquintResolver) ResolveNWithTTL(ctx context.Context, name string, depth int) (path.Path, infd.Duration, error) {
	return resolve(ctx, r, name, depth, "/ipns/")
}

// resolveOnce implements resolver. Decodes the proquint string.
func (r *ProquintResolver) resolveOnce(ctx context.Context, name string) (path.Path, infd.Duration, error) {
	ok, err := proquint.IsProquint(name)
	if err != nil || !ok {
		return "", infd.InfiniteDuration(), errors.New("not a valid proquint string")
	}
	return path.FromString(string(proquint.Decode(name))), infd.InfiniteDuration(), nil
}
