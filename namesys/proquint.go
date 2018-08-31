package namesys

import (
	"errors"
	"time"

	context "context"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	proquint "gx/ipfs/QmYnf27kzqR2cxt6LFZdrAFJuQd6785fTkBvMuEj9EeRxM/proquint"
	path "gx/ipfs/QmdMPBephdLYNESkruDX2hcDTgFYhoCt4LimWhgnomSdV2/go-path"
)

type ProquintResolver struct{}

// Resolve implements Resolver.
func (r *ProquintResolver) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	return resolve(ctx, r, name, opts.ProcessOpts(options), "/ipns/")
}

// resolveOnce implements resolver. Decodes the proquint string.
func (r *ProquintResolver) resolveOnce(ctx context.Context, name string, options *opts.ResolveOpts) (path.Path, time.Duration, error) {
	ok, err := proquint.IsProquint(name)
	if err != nil || !ok {
		return "", 0, errors.New("not a valid proquint string")
	}
	// Return a 0 TTL as caching this result is pointless.
	return path.FromString(string(proquint.Decode(name))), 0, nil
}
