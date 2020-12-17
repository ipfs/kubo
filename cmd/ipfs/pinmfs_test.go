package main

import (
	"context"

	config "github.com/ipfs/go-ipfs-config"
)

type testPinMFSContext struct {
	ctx context.Context
	cfg *config.Config
	err error
}

func (x *testPinMFSContext) Context() context.Context {
	return x.ctx
}

func (x *testPinMFSContext) GetConfigNoCache() (*config.Config, error) {
	return x.cfg, x.err
}

// func TestPinMFS(t *testing.T) {
// 	ctx := &testPinMFSContext{
// 		ctx: context.Background(),
// 		cfg: XXX,
// 		err: XXX,
// 	}
// 	node := XXX
// 	go func() {
// 		pinMFSOnChange(ctx, node)
// 	}()
// 	XXX
// }
