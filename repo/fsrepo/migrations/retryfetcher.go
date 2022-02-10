package migrations

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

type RetryFetcher struct {
	Fetcher
	maxRetries int
}

var _ Fetcher = (*RetryFetcher)(nil)

func NewRetryFetcher(baseFetcher Fetcher, maxRetries int) *RetryFetcher {
	return &RetryFetcher{Fetcher: baseFetcher, maxRetries: maxRetries}
}

func (r *RetryFetcher) Fetch(ctx context.Context, filePath string, writer io.Writer) error {
	var lastErr error
	for i := 0; i < r.maxRetries; i++ {
		var buf bytes.Buffer
		err := r.Fetcher.Fetch(ctx, filePath, &buf)
		if err == nil {
			if _, err := io.Copy(writer, &buf); err != nil {
				return err
			}
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
		lastErr = err
	}
	return fmt.Errorf("exceeded number of retries. last error was %w", lastErr)
}

func (r *RetryFetcher) Close() error {
	return r.Fetcher.Close()
}
