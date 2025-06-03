package kubo

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	config "github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

type testPinMFSContext struct {
	ctx context.Context
	cfg *config.Config
	err error
}

func (x *testPinMFSContext) Context() context.Context {
	return x.ctx
}

func (x *testPinMFSContext) GetConfig() (*config.Config, error) {
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

func isErrorSimilar(e1, e2 error) bool {
	switch {
	case e1 == nil && e2 == nil:
		return true
	case e1 != nil && e2 == nil:
		return false
	case e1 == nil && e2 != nil:
		return false
	default:
		return strings.Contains(e1.Error(), e2.Error()) || strings.Contains(e2.Error(), e1.Error())
	}
}

func TestPinMFSConfigError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*testConfigPollInterval)
	defer cancel()

	cctx := &testPinMFSContext{
		ctx: ctx,
		cfg: nil,
		err: fmt.Errorf("couldn't read config"),
	}
	node := &testPinMFSNode{}

	logReader := logging.NewPipeReader()
	go func() {
		pinMFSOnChange(cctx, testConfigPollInterval, node)
		logReader.Close()
	}()

	level, msg := readLogLine(t, logReader)
	if level != "error" {
		t.Error("expected error to be logged")
	}
	if !isErrorSimilar(errors.New(msg), cctx.err) {
		t.Errorf("error did not propagate")
	}
}

func TestPinMFSRootNodeError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*testConfigPollInterval)
	defer cancel()

	// need at least one config to trigger
	cfg := &config.Config{
		Pinning: config.Pinning{
			RemoteServices: map[string]config.RemotePinningService{
				"A": {
					Policies: config.RemotePinningServicePolicies{
						MFS: config.RemotePinningServiceMFSPolicy{
							Enable: false,
						},
					},
				},
			},
		},
	}

	cctx := &testPinMFSContext{
		ctx: ctx,
		cfg: cfg,
		err: nil,
	}
	node := &testPinMFSNode{
		err: fmt.Errorf("cannot create root node"),
	}
	logReader := logging.NewPipeReader()
	go func() {
		pinMFSOnChange(cctx, testConfigPollInterval, node)
		logReader.Close()
	}()
	level, msg := readLogLine(t, logReader)
	if level != "error" {
		t.Error("expected error to be logged")
	}
	if !isErrorSimilar(errors.New(msg), node.err) {
		t.Errorf("error did not propagate")
	}
}

func TestPinMFSService(t *testing.T) {
	cfgInvalidInterval := &config.Config{
		Pinning: config.Pinning{
			RemoteServices: map[string]config.RemotePinningService{
				"disabled": {
					Policies: config.RemotePinningServicePolicies{
						MFS: config.RemotePinningServiceMFSPolicy{
							Enable: false,
						},
					},
				},
				"invalid_interval": {
					Policies: config.RemotePinningServicePolicies{
						MFS: config.RemotePinningServiceMFSPolicy{
							Enable:        true,
							RepinInterval: "INVALID_INTERVAL",
						},
					},
				},
			},
		},
	}
	cfgValidUnnamed := &config.Config{
		Pinning: config.Pinning{
			RemoteServices: map[string]config.RemotePinningService{
				"valid_unnamed": {
					Policies: config.RemotePinningServicePolicies{
						MFS: config.RemotePinningServiceMFSPolicy{
							Enable:        true,
							PinName:       "",
							RepinInterval: "2s",
						},
					},
				},
			},
		},
	}
	cfgValidNamed := &config.Config{
		Pinning: config.Pinning{
			RemoteServices: map[string]config.RemotePinningService{
				"valid_named": {
					Policies: config.RemotePinningServicePolicies{
						MFS: config.RemotePinningServiceMFSPolicy{
							Enable:        true,
							PinName:       "pin_name",
							RepinInterval: "2s",
						},
					},
				},
			},
		},
	}
	testPinMFSServiceWithError(t, cfgInvalidInterval, "remote pinning service \"invalid_interval\" has invalid MFS.RepinInterval")
	testPinMFSServiceWithError(t, cfgValidUnnamed, "error while listing remote pins: empty response from remote pinning service")
	testPinMFSServiceWithError(t, cfgValidNamed, "error while listing remote pins: empty response from remote pinning service")
}

func testPinMFSServiceWithError(t *testing.T, cfg *config.Config, expectedErrorPrefix string) {
	goctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx := &testPinMFSContext{
		ctx: goctx,
		cfg: cfg,
		err: nil,
	}
	node := &testPinMFSNode{
		err: nil,
	}
	logReader := logging.NewPipeReader()
	go func() {
		pinMFSOnChange(ctx, testConfigPollInterval, node)
		logReader.Close()
	}()
	level, msg := readLogLine(t, logReader)
	if level != "error" {
		t.Error("expected error to be logged")
	}
	if !strings.Contains(msg, expectedErrorPrefix) {
		t.Errorf("expecting error containing %q", expectedErrorPrefix)
	}
}

func readLogLine(t *testing.T, logReader io.Reader) (string, string) {
	t.Helper()

	r := bufio.NewReader(logReader)
	data, err := r.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}

	logInfo := struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}{}
	err = json.Unmarshal(data, &logInfo)
	if err != nil {
		t.Fatal(err)
	}
	return logInfo.Level, logInfo.Msg
}
