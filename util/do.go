package util

import "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

func ContextDo(ctx context.Context, f func() error) error {

	ch := make(chan error)

	go func() {
		select {
		case <-ctx.Done():
		case ch <- f():
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case val := <-ch:
		return val
	}
	return nil
}
