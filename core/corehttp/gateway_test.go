package corehttp

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	core "github.com/jbenet/go-ipfs/core"
	coreunix "github.com/jbenet/go-ipfs/core/coreunix"
	namesys "github.com/jbenet/go-ipfs/namesys"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	repo "github.com/jbenet/go-ipfs/repo"
	config "github.com/jbenet/go-ipfs/repo/config"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

type mockNamesys map[string]string

func (m mockNamesys) Resolve(ctx context.Context, name string) (value u.Key, err error) {
	enc, ok := m[name]
	if !ok {
		return "", namesys.ErrResolveFailed
	}
	dec := b58.Decode(enc)
	if len(dec) == 0 {
		return "", fmt.Errorf("invalid b58 string for name %q: %q", name, enc)
	}
	return u.Key(dec), nil
}

func (m mockNamesys) CanResolve(name string) bool {
	_, ok := m[name]
	return ok
}

func (m mockNamesys) Publish(ctx context.Context, name ci.PrivKey, value u.Key) error {
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
	ns := mockNamesys{}
	n := newNodeWithMockNamesys(t, ns)
	k, err := coreunix.Add(n, strings.NewReader("fnord"))
	if err != nil {
		t.Fatal(err)
	}
	ns["example.com"] = k

	h, err := makeHandler(n,
		IPNSHostnameOption(),
		GatewayOption(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(h)
	defer ts.Close()

	for _, test := range []struct {
		host   string
		path   string
		status int
		text   string
	}{
		{"localhost:5001", "/", http.StatusNotFound, "404 page not found\n"},
		{"localhost:5001", "/" + k, http.StatusNotFound, "404 page not found\n"},
		{"localhost:5001", "/ipfs/" + k, http.StatusOK, "fnord"},
		{"localhost:5001", "/ipns/nxdomain.example.com", http.StatusBadRequest, namesys.ErrResolveFailed.Error()},
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
