package corehttp

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	core "github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	namesys "github.com/ipfs/go-ipfs/namesys"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
)

type mockNamesys map[string]path.Path

func (m mockNamesys) Resolve(ctx context.Context, name string) (value path.Path, err error) {
	return m.ResolveN(ctx, name, namesys.DefaultDepthLimit)
}

func (m mockNamesys) ResolveN(ctx context.Context, name string, depth int) (value path.Path, err error) {
	p, ok := m[name]
	if !ok {
		return "", namesys.ErrResolveFailed
	}
	return p, nil
}

func (m mockNamesys) Publish(ctx context.Context, name ci.PrivKey, value path.Path) error {
	return errors.New("not implemented for mockNamesys")
}

func newNodeWithMockNamesys(t *testing.T, ns mockNamesys) *core.IpfsNode {
	c := config.Config{
		Identity: config.Identity{
			PeerID: "Qmfoo", // required by offline node
		},
	}
	r := &repo.Mock{
		C: c,
		D: testutil.ThreadSafeCloserMapDatastore(),
	}
	n, err := core.NewIPFSNode(context.Background(), core.Offline(r))
	if err != nil {
		t.Fatal(err)
	}
	n.Namesys = ns
	return n
}

func TestGatewayGet(t *testing.T) {
	t.Skip("not sure whats going on here")
	ns := mockNamesys{}
	n := newNodeWithMockNamesys(t, ns)
	k, err := coreunix.Add(n, strings.NewReader("fnord"))
	if err != nil {
		t.Fatal(err)
	}
	ns["example.com"] = path.FromString("/ipfs/" + k)

	h, err := makeHandler(n,
		IPNSHostnameOption(),
		GatewayOption(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(h)
	defer ts.Close()

	t.Log(ts.URL)
	for _, test := range []struct {
		host   string
		path   string
		status int
		text   string
	}{
		{"localhost:5001", "/", http.StatusNotFound, "404 page not found\n"},
		{"localhost:5001", "/" + k, http.StatusNotFound, "404 page not found\n"},
		{"localhost:5001", "/ipfs/" + k, http.StatusOK, "fnord"},
		{"localhost:5001", "/ipns/nxdomain.example.com", http.StatusBadRequest, "Path Resolve error: " + namesys.ErrResolveFailed.Error()},
		{"localhost:5001", "/ipns/example.com", http.StatusOK, "fnord"},
		{"example.com", "/", http.StatusOK, "fnord"},
	} {
		var c http.Client
		r, err := http.NewRequest("GET", ts.URL+test.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Host = test.host
		resp, err := c.Do(r)

		urlstr := "http://" + test.host + test.path
		if err != nil {
			t.Errorf("error requesting %s: %s", urlstr, err)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != test.status {
			t.Errorf("got %d, expected %d from %s", resp.StatusCode, test.status, urlstr)
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("error reading response from %s: %s", urlstr, err)
		}
		if string(body) != test.text {
			t.Errorf("unexpected response body from %s: expected %q; got %q", urlstr, test.text, body)
			continue
		}
	}
}
