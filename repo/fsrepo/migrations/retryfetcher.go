package migrations

import (
	"context"
	"fmt"
)

type RetryFetcher struct {
	Fetcher
	MaxTries int
}

var _ Fetcher = (*RetryFetcher)(nil)

func (r *RetryFetcher) Fetch(ctx context.Context, filePath string) ([]byte, error) {
	var lastErr error
	for i := 0; i < r.MaxTries; i++ {
		out, err := r.Fetcher.Fetch(ctx, filePath)
		if err == nil {
			return out, nil
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		lastErr = err
	}
	return nil, fmt.Errorf("exceeded number of retries. last error was %w", lastErr)
}

func (r *RetryFetcher) Close() error {
	return r.Fetcher.Close()
}
