package namesys

import (
	"strings"
	"time"

	context "context"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	path "gx/ipfs/QmdrpbDgeYH3VxkCciQCJY5LkDYdXtig6unDzQmMxFtWEw/go-path"
)

type resolver interface {
	// resolveOnce looks up a name once (without recursion).
	resolveOnce(ctx context.Context, name string, options *opts.ResolveOpts) (value path.Path, ttl time.Duration, err error)
}

// resolve is a helper for implementing Resolver.ResolveN using resolveOnce.
func resolve(ctx context.Context, r resolver, name string, options *opts.ResolveOpts, prefixes ...string) (path.Path, error) {
	depth := options.Depth
	for {
		p, _, err := r.resolveOnce(ctx, name, options)
		if err != nil {
			return "", err
		}
		log.Debugf("resolved %s to %s", name, p.String())

		matched := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(p.String(), prefix) {
				matched = true
				if len(prefixes) == 1 {
					name = strings.TrimPrefix(p.String(), prefix)
				}
				break
			}
		}

		// Not something we can resolve, return it.
		if !matched {
			return p, nil
		}

		// No more depth left, return it.
		if depth == 1 {
			return p, ErrResolveRecursion
		}

		// Depth could be 0.
		if depth > 1 {
			depth--
		}
	}
}
