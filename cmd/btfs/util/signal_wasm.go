package util

import (
	"context"
	"io"
)

type ctxCloser context.CancelFunc

func (c ctxCloser) Close() error {
	c()
	return nil
}

func SetupInterruptHandler(ctx context.Context) (io.Closer, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return ctxCloser(cancel), ctx
}
