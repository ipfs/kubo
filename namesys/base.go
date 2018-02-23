package namesys

import (
	"strings"

	context "context"

	path "github.com/ipfs/go-ipfs/path"
)

type resolver interface {
	// resolveOnce looks up a name once (without recursion).
	resolveOnce(ctx context.Context, name string, opts *ResolveOpts) (value path.Path, err error)
}

// resolve is a helper for implementing Resolver.ResolveN using resolveOnce.
func resolve(ctx context.Context, r resolver, name string, opts *ResolveOpts, prefixes ...string) (path.Path, error) {
	depth := opts.Depth
	for {
		p, err := r.resolveOnce(ctx, name, opts)
		if err != nil {
			return "", err
		}
		log.Debugf("resolved %s to %s", name, p.String())

		if strings.HasPrefix(p.String(), "/ipfs/") {
			// we've bottomed out with an IPFS path
			return p, nil
		}

		if depth == 1 {
			return p, ErrResolveRecursion
		}

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

		if !matched {
			return p, nil
		}

		if depth > 1 {
			depth--
		}
	}
}
