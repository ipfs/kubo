package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	config "github.com/ipfs/go-ipfs-config"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
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

type testPinMFSNode struct {
	err error
}

func (x *testPinMFSNode) RootNode() (ipld.Node, error) {
	return merkledag.NewRawNode([]byte{0x01}), x.err
}

func (x *testPinMFSNode) Identity() peer.ID {
	return peer.ID("test_id")
}

func (x *testPinMFSNode) PeerHost() host.Host {
	return nil
}

var testConfigPollInterval = time.Second

func TestPinMFSConfigError(t *testing.T) {
	ctx := &testPinMFSContext{
		ctx: context.Background(),
		cfg: nil,
		err: fmt.Errorf("couldn't read config"),
	}
	node := &testPinMFSNode{}
	errCh := make(chan error)
	go func() {
		pinMFSOnChange(testConfigPollInterval, ctx, node, errCh)
	}()
	if <-errCh != ctx.err {
		t.Errorf("error did not propagate")
	}
	if <-errCh != ctx.err {
		t.Errorf("error did not propagate")
	}
}

func TestPinMFSRootNodeError(t *testing.T) {
	ctx := &testPinMFSContext{
		ctx: context.Background(),
		cfg: &config.Config{
			Pinning: config.Pinning{},
		},
		err: nil,
	}
	node := &testPinMFSNode{
		err: fmt.Errorf("cannot create root node"),
	}
	errCh := make(chan error)
	go func() {
		pinMFSOnChange(testConfigPollInterval, ctx, node, errCh)
	}()
	if <-errCh != node.err {
		t.Errorf("error did not propagate")
	}
	if <-errCh != node.err {
		t.Errorf("error did not propagate")
	}
}
