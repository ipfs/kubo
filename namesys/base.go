package namesys

import (
	"context"
	"strings"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"

	path "gx/ipfs/QmdrpbDgeYH3VxkCciQCJY5LkDYdXtig6unDzQmMxFtWEw/go-path"
)

type onceResult struct {
	value path.Path
	ttl   time.Duration
	err   error
}

type resolver interface {
	resolveOnceAsync(ctx context.Context, name string, options opts.ResolveOpts) <-chan onceResult
}

// resolve is a helper for implementing Resolver.ResolveN using resolveOnce.
func resolve(ctx context.Context, r resolver, name string, options opts.ResolveOpts) (path.Path, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := ErrResolveFailed
	var p path.Path

	resCh := resolveAsync(ctx, r, name, options)

	for res := range resCh {
		p, err = res.Path, res.Err
		if err != nil {
			break
		}
	}

	return p, err
}

func resolveAsync(ctx context.Context, r resolver, name string, options opts.ResolveOpts) <-chan Result {
	resCh := r.resolveOnceAsync(ctx, name, options)
	depth := options.Depth
	outCh := make(chan Result, 1)

	go func() {
		defer close(outCh)
		var subCh <-chan Result
		var cancelSub context.CancelFunc

		for {
			select {
			case res, ok := <-resCh:
				if !ok {
					resCh = nil
					break
				}

				if res.err != nil {
					outCh <- Result{Err: res.err}
					if cancelSub != nil {
						cancelSub()
					}
					return
				}
				log.Debugf("resolved %s to %s", name, res.value.String())
				if !strings.HasPrefix(res.value.String(), ipnsPrefix) {
					outCh <- Result{Path: res.value}
					break
				}

				if depth == 1 {
					outCh <- Result{Path: res.value, Err: ErrResolveRecursion}
					break
				}

				subopts := options
				if subopts.Depth > 1 {
					subopts.Depth--
				}

				var subCtx context.Context
				if cancelSub != nil {
					// Cancel previous recursive resolve since it won't be used anyways
					cancelSub()
				}
				subCtx, cancelSub = context.WithCancel(ctx)

				p := strings.TrimPrefix(res.value.String(), ipnsPrefix)
				subCh = resolveAsync(subCtx, r, p, subopts)
			case res, ok := <-subCh:
				if !ok {
					subCh = nil
					break
				}

				select {
				case outCh <- res:
				case <-ctx.Done():
					if cancelSub != nil {
						cancelSub()
					}
					return
				}
			case <-ctx.Done():
				if cancelSub != nil {
					cancelSub()
				}
				return
			}
			if resCh == nil && subCh == nil {
				if cancelSub != nil {
					cancelSub()
				}
				return
			}
		}
	}()
	return outCh
}
