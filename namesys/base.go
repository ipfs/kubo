package namesys

import (
	"context"
	"strings"
	"time"

	path "github.com/ipfs/go-path"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	opts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
)

type onceResult struct {
	value path.Path
	proof [][]byte
	ttl   time.Duration
	err   error
}

type resolver interface {
	resolveOnceAsync(ctx context.Context, name string, needsProof bool, options opts.ResolveOpts) <-chan onceResult
}

// resolve is a helper for implementing Resolver.ResolveN using resolveOnce.
func resolve(ctx context.Context, r resolver, name string, options opts.ResolveOpts) (path.Path, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		p     path.Path
		proof [][]byte
		err   = ErrResolveFailed
	)

	resCh := resolveAsync(ctx, r, name, options)

	for res := range resCh {
		p, proof, err = res.Path, res.Proof, res.Err
		if err != nil {
			break
		}
	}

	if pw, ok := ctx.Value("proxy-preamble").(coreiface.ProofWriter); ok {
		for _, p := range proof {
			pw.WriteChunk(p)
		}
	}
	return p, err
}

func resolveAsync(ctx context.Context, r resolver, name string, options opts.ResolveOpts) <-chan Result {
	_, needsProof := ctx.Value("proxy-preamble").(coreiface.ProofWriter)
	resCh := r.resolveOnceAsync(ctx, name, needsProof, options)
	depth := options.Depth
	outCh := make(chan Result, 1)

	go func() {
		defer close(outCh)
		var subCh <-chan Result
		var cancelSub context.CancelFunc
		var proofStub [][]byte
		defer func() {
			if cancelSub != nil {
				cancelSub()
			}
		}()

		for {
			select {
			case res, ok := <-resCh:
				if !ok {
					resCh = nil
					break
				}

				if res.err != nil {
					emitResult(ctx, outCh, Result{Err: res.err})
					return
				}
				log.Debugf("resolved %s to %s", name, res.value.String())
				if !strings.HasPrefix(res.value.String(), ipnsPrefix) {
					emitResult(ctx, outCh, Result{Path: res.value, Proof: res.proof})
					break
				}

				if depth == 1 {
					emitResult(ctx, outCh, Result{Path: res.value, Err: ErrResolveRecursion})
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
				_ = cancelSub

				p := strings.TrimPrefix(res.value.String(), ipnsPrefix)
				subCh = resolveAsync(subCtx, r, p, subopts)
				proofStub = res.proof
			case res, ok := <-subCh:
				if !ok {
					subCh = nil
					break
				}

				// We don't bother returning here in case of context timeout as there is
				// no good reason to do that, and we may still be able to emit a result
				res.Proof = append(proofStub, res.Proof...)
				emitResult(ctx, outCh, res)
			case <-ctx.Done():
				return
			}
			if resCh == nil && subCh == nil {
				return
			}
		}
	}()
	return outCh
}

func emitResult(ctx context.Context, outCh chan<- Result, r Result) {
	select {
	case outCh <- r:
	case <-ctx.Done():
	}
}
