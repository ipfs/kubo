package namesys

import (
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	path "github.com/ipfs/go-ipfs/path"
	infd "github.com/ipfs/go-ipfs/util/infduration"
)

type resolver interface {
	// resolveOnce looks up a name once (without recursion).  It also
	// returns a time-to-live value which indicates the maximum amount of
	// time the result (whether a success or an error) may be cached.
	resolveOnce(ctx context.Context, name string) (value path.Path, ttl infd.Duration, err error)
}

// resolve is a helper for implementing Resolver.ResolveNWithTTL using
// resolveOnce.
func resolve(ctx context.Context, r resolver, name string, depth int, prefixes ...string) (path.Path, infd.Duration, error) {
	// Start with a long TTL.
	ttl := infd.InfiniteDuration()

	for {
		p, resTTL, err := r.resolveOnce(ctx, name)
		// Use the lowest TTL reported by the resolveOnce invocations.
		ttl = infd.Min(ttl, resTTL)
		if err != nil {
			log.Warningf("Could not resolve %s", name)
			return "", ttl, err
		}
		log.Debugf("Resolved %s to %s (TTL %v -> %v)", name, p.String(), resTTL, ttl)

		if strings.HasPrefix(p.String(), "/ipfs/") {
			// we've bottomed out with an IPFS path
			return p, ttl, nil
		}

		if depth == 1 {
			return p, ttl, ErrResolveRecursion
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
			return p, ttl, nil
		}

		if depth > 1 {
			depth--
		}
	}
}
