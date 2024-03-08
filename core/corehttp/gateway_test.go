package corehttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ipfs/boxo/namesys"
	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/repo"
	"github.com/stretchr/testify/assert"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/kubo/config"
	iface "github.com/ipfs/kubo/core/coreiface"
	ci "github.com/libp2p/go-libp2p/core/crypto"
)

type mockNamesys map[string]path.Path

func (m mockNamesys) Resolve(ctx context.Context, p path.Path, opts ...namesys.ResolveOption) (namesys.Result, error) {
	cfg := namesys.DefaultResolveOptions()
	for _, o := range opts {
		o(&cfg)
	}
	depth := cfg.Depth
	if depth == namesys.UnlimitedDepth {
		// max uint
		depth = ^uint(0)
	}
	var (
		value path.Path
	)
	name := path.SegmentsToString(p.Segments()[:2]...)
	for strings.HasPrefix(name, "/ipns/") {
		if depth == 0 {
			return namesys.Result{Path: value}, namesys.ErrResolveRecursion
		}
		depth--

		v, ok := m[name]
		if !ok {
			return namesys.Result{}, namesys.ErrResolveFailed
		}
		value = v
		name = value.String()
	}

	value, err := path.Join(value, p.Segments()[2:]...)
	return namesys.Result{Path: value}, err
}

func (m mockNamesys) ResolveAsync(ctx context.Context, p path.Path, opts ...namesys.ResolveOption) <-chan namesys.AsyncResult {
	out := make(chan namesys.AsyncResult, 1)
	res, err := m.Resolve(ctx, p, opts...)
	out <- namesys.AsyncResult{Path: res.Path, TTL: res.TTL, LastMod: res.LastMod, Err: err}
	close(out)
	return out
}

func (m mockNamesys) Publish(ctx context.Context, name ci.PrivKey, value path.Path, opts ...namesys.PublishOption) error {
	return errors.New("not implemented for mockNamesys")
}

func (m mockNamesys) GetResolver(subs string) (namesys.Resolver, bool) {
	return nil, false
}

func newNodeWithMockNamesys(ns mockNamesys) (*core.IpfsNode, error) {
	c := config.Config{
		Identity: config.Identity{
			PeerID: "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe", // required by offline node
		},
	}
	r := &repo.Mock{
		C: c,
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
	}
	n, err := core.NewNode(context.Background(), &core.BuildCfg{Repo: r})
	if err != nil {
		return nil, err
	}
	n.Namesys = ns
	return n, nil
}

type delegatedHandler struct {
	http.Handler
}

func (dh *delegatedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dh.Handler.ServeHTTP(w, r)
}

func doWithoutRedirect(req *http.Request) (*http.Response, error) {
	tag := "without-redirect"
	c := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New(tag)
		},
	}
	res, err := c.Do(req)
	if err != nil && !strings.Contains(err.Error(), tag) {
		return nil, err
	}
	return res, nil
}

func newTestServerAndNode(t *testing.T, ns mockNamesys) (*httptest.Server, iface.CoreAPI, context.Context) {
	n, err := newNodeWithMockNamesys(ns)
	if err != nil {
		t.Fatal(err)
	}

	// need this variable here since we need to construct handler with
	// listener, and server with handler. yay cycles.
	dh := &delegatedHandler{}
	ts := httptest.NewServer(dh)
	t.Cleanup(func() { ts.Close() })

	dh.Handler, err = MakeHandler(n,
		ts.Listener,
		HostnameOption(),
		GatewayOption("/ipfs", "/ipns"),
		VersionOption(),
	)
	if err != nil {
		t.Fatal(err)
	}

	api, err := coreapi.NewCoreAPI(n)
	if err != nil {
		t.Fatal(err)
	}

	return ts, api, n.Context()
}

func TestVersion(t *testing.T) {
	version.CurrentCommit = "theshortcommithash"

	ns := mockNamesys{}
	ts, _, _ := newTestServerAndNode(t, ns)
	t.Logf("test server url: %s", ts.URL)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/version", nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("error reading response: %s", err)
	}
	s := string(body)

	if !strings.Contains(s, "Commit: theshortcommithash") {
		t.Fatalf("response doesn't contain commit:\n%s", s)
	}

	if !strings.Contains(s, "Client Version: "+version.GetUserAgentVersion()) {
		t.Fatalf("response doesn't contain client version:\n%s", s)
	}
}

func TestDeserializedResponsesInheritance(t *testing.T) {
	for _, testCase := range []struct {
		globalSetting          config.Flag
		gatewaySetting         config.Flag
		expectedGatewaySetting bool
	}{
		{config.True, config.Default, true},
		{config.False, config.Default, false},
		{config.False, config.True, true},
		{config.True, config.False, false},
	} {
		c := config.Config{
			Identity: config.Identity{
				PeerID: "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe", // required by offline node
			},
			Gateway: config.Gateway{
				DeserializedResponses: testCase.globalSetting,
				PublicGateways: map[string]*config.GatewaySpec{
					"example.com": {
						DeserializedResponses: testCase.gatewaySetting,
					},
				},
			},
		}
		r := &repo.Mock{
			C: c,
			D: syncds.MutexWrap(datastore.NewMapDatastore()),
		}
		n, err := core.NewNode(context.Background(), &core.BuildCfg{Repo: r})
		assert.NoError(t, err)

		gwCfg, _, err := getGatewayConfig(n)
		assert.NoError(t, err)

		assert.Contains(t, gwCfg.PublicGateways, "example.com")
		assert.Equal(t, testCase.expectedGatewaySetting, gwCfg.PublicGateways["example.com"].DeserializedResponses)
	}
}
