package namesys

import (
	"context"
	"errors"

	path "gx/ipfs/QmQ3YSqfxunT5QBg6KBVskKyRE26q6hjSMyhpxchpm7jEN/go-path"
	opts "gx/ipfs/QmWqb6eEpQ2qtu2jmcDWJXebP7YS14fwor8562g795ZxjH/interface-go-ipfs-core/options/namesys"
	proquint "gx/ipfs/QmYnf27kzqR2cxt6LFZdrAFJuQd6785fTkBvMuEj9EeRxM/proquint"
)

type ProquintResolver struct{}

// Resolve implements Resolver.
func (r *ProquintResolver) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	return resolve(ctx, r, name, opts.ProcessOpts(options))
}

// resolveOnce implements resolver. Decodes the proquint string.
func (r *ProquintResolver) resolveOnceAsync(ctx context.Context, name string, options opts.ResolveOpts) <-chan onceResult {
	out := make(chan onceResult, 1)
	defer close(out)

	ok, err := proquint.IsProquint(name)
	if err != nil || !ok {
		out <- onceResult{err: errors.New("not a valid proquint string")}
		return out
	}
	// Return a 0 TTL as caching this result is pointless.
	out <- onceResult{value: path.FromString(string(proquint.Decode(name)))}
	return out
}
