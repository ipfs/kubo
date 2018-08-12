package corehttp

import (
	"context"
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	version "github.com/ipfs/go-ipfs"
	core "github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	namesys "github.com/ipfs/go-ipfs/namesys"
	nsopts "github.com/ipfs/go-ipfs/namesys/opts"
	repo "github.com/ipfs/go-ipfs/repo"

	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	config "gx/ipfs/QmQSG7YCizeUH2bWatzp6uK9Vm3m7LA5jpxGa9QqgpNKw4/go-ipfs-config"
	dag "gx/ipfs/QmQzSpSjkdGHW6WFBhUG6P3t9K8yv7iucucT1cQaqJ6tgd/go-merkledag"
	id "gx/ipfs/QmUDzeFgYrRmHL2hUB6NZmqcBVQtUzETwmFRUc9onfSSHr/go-libp2p/p2p/protocol/identify"
	datastore "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	syncds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore/sync"
	path "gx/ipfs/QmWMcvZbNvk5codeqbm7L89C9kqSwka4KaHnDb8HRnxsSL/go-path"
)

// `ipfs object new unixfs-dir`
var emptyDir = "/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"

type mockNamesys map[string]path.Path

func (m mockNamesys) Resolve(ctx context.Context, name string, opts ...nsopts.ResolveOpt) (value path.Path, err error) {
	cfg := nsopts.DefaultResolveOpts()
	for _, o := range opts {
		o(cfg)
	}
	depth := cfg.Depth
	if depth == nsopts.UnlimitedDepth {
		depth = math.MaxUint64
	}
	for strings.HasPrefix(name, "/ipns/") {
		if depth <= 0 {
			return value, namesys.ErrResolveRecursion
		}
		depth--

		var ok bool
		value, ok = m[name]
		if !ok {
			return "", namesys.ErrResolveFailed
		}
		name = value.String()
	}
	return value, nil
}

func (m mockNamesys) Publish(ctx context.Context, name ci.PrivKey, value path.Path) error {
	return errors.New("not implemented for mockNamesys")
}

func (m mockNamesys) PublishWithEOL(ctx context.Context, name ci.PrivKey, value path.Path, _ time.Time) error {
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

func newTestServerAndNode(t *testing.T, ns mockNamesys) (*httptest.Server, *core.IpfsNode) {
	n, err := newNodeWithMockNamesys(ns)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := n.Repo.Config()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Gateway.PathPrefixes = []string{"/good-prefix"}

	// need this variable here since we need to construct handler with
	// listener, and server with handler. yay cycles.
	dh := &delegatedHandler{}
	ts := httptest.NewServer(dh)

	dh.Handler, err = makeHandler(n,
		ts.Listener,
		VersionOption(),
		IPNSHostnameOption(),
		GatewayOption(false, "/ipfs", "/ipns"),
	)
	if err != nil {
		t.Fatal(err)
	}

	return ts, n
}

func TestGatewayGet(t *testing.T) {
	ns := mockNamesys{}
	ts, n := newTestServerAndNode(t, ns)
	defer ts.Close()

	k, err := coreunix.Add(n, strings.NewReader("fnord"))
	if err != nil {
		t.Fatal(err)
	}
	ns["/ipns/example.com"] = path.FromString("/ipfs/" + k)
	ns["/ipns/working.example.com"] = path.FromString("/ipfs/" + k)
	ns["/ipns/double.example.com"] = path.FromString("/ipns/working.example.com")
	ns["/ipns/triple.example.com"] = path.FromString("/ipns/double.example.com")
	ns["/ipns/broken.example.com"] = path.FromString("/ipns/" + k)

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
		{"localhost:5001", "/ipns/nxdomain.example.com", http.StatusNotFound, "ipfs resolve -r /ipns/nxdomain.example.com: " + namesys.ErrResolveFailed.Error() + "\n"},
		{"localhost:5001", "/ipns/%0D%0A%0D%0Ahello", http.StatusNotFound, "ipfs resolve -r /ipns/%0D%0A%0D%0Ahello: " + namesys.ErrResolveFailed.Error() + "\n"},
		{"localhost:5001", "/ipns/example.com", http.StatusOK, "fnord"},
		{"example.com", "/", http.StatusOK, "fnord"},

		{"working.example.com", "/", http.StatusOK, "fnord"},
		{"double.example.com", "/", http.StatusOK, "fnord"},
		{"triple.example.com", "/", http.StatusOK, "fnord"},
		{"working.example.com", "/ipfs/" + k, http.StatusNotFound, "ipfs resolve -r /ipns/working.example.com/ipfs/" + k + ": no link by that name\n"},
		{"broken.example.com", "/", http.StatusNotFound, "ipfs resolve -r /ipns/broken.example.com/: " + namesys.ErrResolveFailed.Error() + "\n"},
		{"broken.example.com", "/ipfs/" + k, http.StatusNotFound, "ipfs resolve -r /ipns/broken.example.com/ipfs/" + k + ": " + namesys.ErrResolveFailed.Error() + "\n"},
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

func TestIPNSHostnameRedirect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ns := mockNamesys{}
	ts, n := newTestServerAndNode(t, ns)
	t.Logf("test server url: %s", ts.URL)
	defer ts.Close()

	// create /ipns/example.net/foo/index.html
	_, dagn1, err := coreunix.AddWrapped(n, strings.NewReader("_"), "_")
	if err != nil {
		t.Fatal(err)
	}

	_, dagn2, err := coreunix.AddWrapped(n, strings.NewReader("_"), "index.html")
	if err != nil {
		t.Fatal(err)
	}

	dagn1.(*dag.ProtoNode).AddNodeLink("foo", dagn2)
	if err != nil {
		t.Fatal(err)
	}

	err = n.DAG.Add(ctx, dagn2)
	if err != nil {
		t.Fatal(err)
	}

	err = n.DAG.Add(ctx, dagn1)
	if err != nil {
		t.Fatal(err)
	}

	k := dagn1.Cid()
	t.Logf("k: %s\n", k)
	ns["/ipns/example.net"] = path.FromString("/ipfs/" + k.String())

	// make request to directory containing index.html
	req, err := http.NewRequest("GET", ts.URL+"/foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "example.net"

	res, err := doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}

	// expect 302 redirect to same path, but with trailing slash
	if res.StatusCode != 302 {
		t.Errorf("status is %d, expected 302", res.StatusCode)
	}
	hdr := res.Header["Location"]
	if len(hdr) < 1 {
		t.Errorf("location header not present")
	} else if hdr[0] != "/foo/" {
		t.Errorf("location header is %v, expected /foo/", hdr[0])
	}

	// make request with prefix to directory containing index.html
	req, err = http.NewRequest("GET", ts.URL+"/foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "example.net"
	req.Header.Set("X-Ipfs-Gateway-Prefix", "/good-prefix")

	res, err = doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}

	// expect 302 redirect to same path, but with prefix and trailing slash
	if res.StatusCode != 302 {
		t.Errorf("status is %d, expected 302", res.StatusCode)
	}
	hdr = res.Header["Location"]
	if len(hdr) < 1 {
		t.Errorf("location header not present")
	} else if hdr[0] != "/good-prefix/foo/" {
		t.Errorf("location header is %v, expected /good-prefix/foo/", hdr[0])
	}
}

func TestIPNSHostnameBacklinks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ns := mockNamesys{}
	ts, n := newTestServerAndNode(t, ns)
	t.Logf("test server url: %s", ts.URL)
	defer ts.Close()

	// create /ipns/example.net/foo/
	_, dagn1, err := coreunix.AddWrapped(n, strings.NewReader("1"), "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, dagn2, err := coreunix.AddWrapped(n, strings.NewReader("2"), "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, dagn3, err := coreunix.AddWrapped(n, strings.NewReader("3"), "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	dagn2.(*dag.ProtoNode).AddNodeLink("bar", dagn3)
	dagn1.(*dag.ProtoNode).AddNodeLink("foo? #<'", dagn2)
	if err != nil {
		t.Fatal(err)
	}

	err = n.DAG.Add(ctx, dagn3)
	if err != nil {
		t.Fatal(err)
	}
	err = n.DAG.Add(ctx, dagn2)
	if err != nil {
		t.Fatal(err)
	}
	err = n.DAG.Add(ctx, dagn1)
	if err != nil {
		t.Fatal(err)
	}

	k := dagn1.Cid()
	t.Logf("k: %s\n", k)
	ns["/ipns/example.net"] = path.FromString("/ipfs/" + k.String())

	// make request to directory listing
	req, err := http.NewRequest("GET", ts.URL+"/foo%3F%20%23%3C%27/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "example.net"

	res, err := doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}

	// expect correct backlinks
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("error reading response: %s", err)
	}
	s := string(body)
	t.Logf("body: %s\n", string(body))

	if !strings.Contains(s, "Index of /foo? #&lt;&#39;/") {
		t.Fatalf("expected a path in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/\">") {
		t.Fatalf("expected backlink in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/foo%3F%20%23%3C%27/file.txt\">") {
		t.Fatalf("expected file in directory listing")
	}

	// make request to directory listing at root
	req, err = http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "example.net"

	res, err = doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}

	// expect correct backlinks at root
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("error reading response: %s", err)
	}
	s = string(body)
	t.Logf("body: %s\n", string(body))

	if !strings.Contains(s, "Index of /") {
		t.Fatalf("expected a path in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/\">") {
		t.Fatalf("expected backlink in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/file.txt\">") {
		t.Fatalf("expected file in directory listing")
	}

	// make request to directory listing
	req, err = http.NewRequest("GET", ts.URL+"/foo%3F%20%23%3C%27/bar/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "example.net"

	res, err = doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}

	// expect correct backlinks
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("error reading response: %s", err)
	}
	s = string(body)
	t.Logf("body: %s\n", string(body))

	if !strings.Contains(s, "Index of /foo? #&lt;&#39;/bar/") {
		t.Fatalf("expected a path in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/foo%3F%20%23%3C%27/\">") {
		t.Fatalf("expected backlink in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/foo%3F%20%23%3C%27/bar/file.txt\">") {
		t.Fatalf("expected file in directory listing")
	}

	// make request to directory listing with prefix
	req, err = http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "example.net"
	req.Header.Set("X-Ipfs-Gateway-Prefix", "/good-prefix")

	res, err = doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}

	// expect correct backlinks with prefix
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("error reading response: %s", err)
	}
	s = string(body)
	t.Logf("body: %s\n", string(body))

	if !strings.Contains(s, "Index of /good-prefix") {
		t.Fatalf("expected a path in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/good-prefix/\">") {
		t.Fatalf("expected backlink in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/good-prefix/file.txt\">") {
		t.Fatalf("expected file in directory listing")
	}

	// make request to directory listing with illegal prefix
	req, err = http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "example.net"
	req.Header.Set("X-Ipfs-Gateway-Prefix", "/bad-prefix")

	// make request to directory listing with evil prefix
	req, err = http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "example.net"
	req.Header.Set("X-Ipfs-Gateway-Prefix", "//good-prefix/foo")

	res, err = doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}

	// expect correct backlinks without illegal prefix
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("error reading response: %s", err)
	}
	s = string(body)
	t.Logf("body: %s\n", string(body))

	if !strings.Contains(s, "Index of /") {
		t.Fatalf("expected a path in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/\">") {
		t.Fatalf("expected backlink in directory listing")
	}
	if !strings.Contains(s, "<a href=\"/file.txt\">") {
		t.Fatalf("expected file in directory listing")
	}
}

func TestCacheControlImmutable(t *testing.T) {
	ts, _ := newTestServerAndNode(t, nil)
	t.Logf("test server url: %s", ts.URL)
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+emptyDir+"/", nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}

	// check the immutable tag isn't set
	hdrs, ok := res.Header["Cache-Control"]
	if ok {
		for _, hdr := range hdrs {
			if strings.Contains(hdr, "immutable") {
				t.Fatalf("unexpected Cache-Control: immutable on directory listing: %s", hdr)
			}
		}
	}
}

func TestGoGetSupport(t *testing.T) {
	ts, _ := newTestServerAndNode(t, nil)
	t.Logf("test server url: %s", ts.URL)
	defer ts.Close()

	// mimic go-get
	req, err := http.NewRequest("GET", ts.URL+emptyDir+"?go-get=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != 200 {
		t.Errorf("status is %d, expected 200", res.StatusCode)
	}
}

func TestVersion(t *testing.T) {
	version.CurrentCommit = "theshortcommithash"

	ns := mockNamesys{}
	ts, _ := newTestServerAndNode(t, ns)
	t.Logf("test server url: %s", ts.URL)
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL+"/version", nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := doWithoutRedirect(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("error reading response: %s", err)
	}
	s := string(body)

	if !strings.Contains(s, "Commit: theshortcommithash") {
		t.Fatalf("response doesn't contain commit:\n%s", s)
	}

	if !strings.Contains(s, "Client Version: "+id.ClientVersion) {
		t.Fatalf("response doesn't contain client version:\n%s", s)
	}

	if !strings.Contains(s, "Protocol Version: "+id.LibP2PVersion) {
		t.Fatalf("response doesn't contain protocol version:\n%s", s)
	}
}
