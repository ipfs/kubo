package kubo

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	pinclient "github.com/ipfs/boxo/pinning/remote/client"
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

// fakePinningService is a remote pinning service that holds at most one pin
// and reports it with whatever status the test sets.
type fakePinningService struct {
	mu       sync.Mutex
	cid      string
	status   string // "" means there is no pin
	lists    int
	adds     int
	replaces int
}

func (s *fakePinningService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pin := func() map[string]any {
		return map[string]any{
			"requestid": "req-1",
			"status":    s.status,
			"created":   "2021-01-01T00:00:00Z",
			"pin":       map[string]any{"cid": s.cid},
			"delegates": []string{"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWHDDGfNebz3WBtmBA5ZoEiy56xFRw43kV3tDpkfR3kkGa"},
		}
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/pins":
		s.lists++
		results := []any{}
		if s.status != "" {
			results = append(results, pin())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"count": len(results), "results": results})
	case r.Method == http.MethodPost && r.URL.Path == "/pins":
		s.adds++
		s.status = "queued"
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(pin())
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/pins/"):
		s.replaces++
		s.status = "queued"
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(pin())
	default:
		http.NotFound(w, r)
	}
}

func (s *fakePinningService) set(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *fakePinningService) counts() (lists, adds, replaces int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lists, s.adds, s.replaces
}

func TestPinMFSRechecksPinUntilPinned(t *testing.T) {
	node := &testPinMFSNode{}
	root, err := node.RootNode()
	if err != nil {
		t.Fatal(err)
	}
	svc := &fakePinningService{cid: root.Cid().String()}
	srv := httptest.NewServer(svc)
	defer srv.Close()

	const repinInterval = 50 * time.Millisecond
	cfg := &config.Config{
		Pinning: config.Pinning{
			RemoteServices: map[string]config.RemotePinningService{
				"svc": {
					API: config.RemotePinningServiceAPI{Endpoint: srv.URL, Key: "key"},
					Policies: config.RemotePinningServicePolicies{
						MFS: config.RemotePinningServiceMFSPolicy{
							Enable:        true,
							RepinInterval: repinInterval.String(),
						},
					},
				},
			},
		},
	}

	logReader := logging.NewPipeReader()
	defer logReader.Close()
	errorLogs := make(chan string, 16)
	go func() {
		r := bufio.NewReader(logReader)
		for {
			data, err := r.ReadBytes('\n')
			if err != nil {
				return
			}
			var line struct {
				Level string `json:"level"`
				Msg   string `json:"msg"`
			}
			if json.Unmarshal(data, &line) == nil && line.Level == "error" {
				errorLogs <- line.Msg
			}
		}
	}()

	ctx := context.Background()
	lastPins := map[string]lastPin{}
	expect := func(step string, wantLists, wantAdds, wantReplaces int, wantStatus pinclient.Status) {
		t.Helper()
		lists, adds, replaces := svc.counts()
		if lists != wantLists || adds != wantAdds || replaces != wantReplaces {
			t.Fatalf("%s: got lists=%d adds=%d replaces=%d, want %d %d %d", step, lists, adds, replaces, wantLists, wantAdds, wantReplaces)
		}
		if got := lastPins["svc"].Status; got != wantStatus {
			t.Fatalf("%s: got last status %q, want %q", step, got, wantStatus)
		}
	}

	pinAllMFS(ctx, node, cfg, lastPins)
	expect("first pin", 1, 1, 0, pinclient.StatusQueued)

	// The pin is not confirmed yet, but MFS.RepinInterval has not passed.
	pinAllMFS(ctx, node, cfg, lastPins)
	expect("within interval", 1, 1, 0, pinclient.StatusQueued)

	// The service failed the pin: it is logged as an error and requested again.
	svc.set("failed")
	time.Sleep(repinInterval + 10*time.Millisecond)
	pinAllMFS(ctx, node, cfg, lastPins)
	expect("after failure", 2, 1, 1, pinclient.StatusQueued)
	select {
	case msg := <-errorLogs:
		if !strings.Contains(msg, `"failed"`) || !strings.Contains(msg, svc.cid) {
			t.Fatalf("unexpected error log: %s", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("expected the failed pin to be logged as an error")
	}

	// Once the service reports the pin as pinned, it is no longer checked.
	svc.set("pinned")
	time.Sleep(repinInterval + 10*time.Millisecond)
	pinAllMFS(ctx, node, cfg, lastPins)
	expect("pinned", 3, 1, 1, pinclient.StatusPinned)

	time.Sleep(repinInterval + 10*time.Millisecond)
	pinAllMFS(ctx, node, cfg, lastPins)
	expect("after pinned", 3, 1, 1, pinclient.StatusPinned)
}
