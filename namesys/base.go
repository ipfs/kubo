package namesys

import (
	"strings"
	"time"

	context "context"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	path "gx/ipfs/QmcjwUb36Z16NJkvDX6ccXPqsFswo6AsRXynyXcLLCphV2/go-path"
)

type onceResult struct {
	value path.Path
	ttl   time.Duration
	err   error
}

type resolver interface {
	// resolveOnce looks up a name once (without recursion).
	resolveOnce(ctx context.Context, name string, options opts.ResolveOpts) (value path.Path, ttl time.Duration, err error)

	resolveOnceAsync(ctx context.Context, name string, options opts.ResolveOpts) <-chan onceResult
}

// resolve is a helper for implementing Resolver.ResolveN using resolveOnce.
func resolve(ctx context.Context, r resolver, name string, options opts.ResolveOpts, prefix string) (path.Path, error) {
	depth := options.Depth
	for {
		p, _, err := r.resolveOnce(ctx, name, options)
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

		if !strings.HasPrefix(p.String(), prefix) {
			return p, nil
		}
		name = strings.TrimPrefix(p.String(), prefix)

		if depth > 1 {
			depth--
		}
	}
}

//TODO:
// - better error handling
func resolveAsyncDo(ctx context.Context, r resolver, name string, options opts.ResolveOpts, prefix string) <-chan Result {
	resCh := r.resolveOnceAsync(ctx, name, options)
	depth := options.Depth
	outCh := make(chan Result)

	go func() {
		defer close(outCh)
		var subCh <-chan Result
		var cancelSub context.CancelFunc

		for {
			select {
			case res, ok := <-resCh:
				if res.err != nil {
					outCh <- Result{err: res.err}
					return
				}
				if !ok {
					resCh = nil
					continue
				}
				log.Debugf("resolved %s to %s", name, res.value.String())
				if strings.HasPrefix(res.value.String(), "/ipfs/") {
					outCh <- Result{err: res.err}
					continue
				}
				p := strings.TrimPrefix(res.value.String(), prefix)

				if depth == 1 {
					outCh <- Result{err: ErrResolveRecursion}
					continue
				}

				subopts := options
				if subopts.Depth > 1 {
					subopts.Depth--
				}

				var subCtx context.Context
				if subCh != nil {
					// Cancel previous recursive resolve since it won't be used anyways
					cancelSub()
				}
				subCtx, cancelSub = context.WithCancel(ctx)

				subCh = resolveAsyncDo(subCtx, r, p, subopts, prefix)
			case res, ok := <-subCh:
				if res.err != nil {
					outCh <- Result{err: res.err}
					return
				}
				if !ok {
					subCh = nil
					continue
				}
				outCh <- res
			case <-ctx.Done():
			}
		}
	}()
	return outCh
}

func resolveAsync(ctx context.Context, r resolver, name string, options opts.ResolveOpts, prefix string) <-chan Result {
	return resolveAsyncDo(ctx, r, name, options, prefix)
}
