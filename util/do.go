package util

import "code.google.com/p/go.net/context"

func Do(ctx context.Context, f func() error) error {
	ch := make(chan error)
	go func() { ch <- f() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case val := <-ch:
		return val
	}
	return nil
}
