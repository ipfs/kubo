package corehttp

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ipfs/go-ipfs/assets"
	"github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/mock"

	"github.com/cheekybits/is"
)

type testSession struct {
	key *key.Key
	mn  *core.IpfsNode
	hc  http.Client
}

func newTestSession(t *testing.T, writable bool) *testSession {
	mn, err := coremock.NewMockNode()
	if err != nil {
		t.Fatalf("coremock.NewMockNode() failed: %s", err)
	}

	tk, err := assets.SeedInitDocs(mn)
	if err != nil {
		t.Fatalf("assets.SeedInitDocs() failed: %s", err)
	}

	gwh, err := newGatewayHandler(mn, GatewayConfig{Writable: writable})
	if err != nil {
		t.Fatalf("newGatewayHandler() failed: %s", err)
	}
	serveMux := http.NewServeMux()
	serveMux.Handle("/", gwh)

	return &testSession{
		key: tk,
		mn:  mn,
		hc:  http.Client{Transport: (*muxTransport)(serveMux)},
	}
}

func TestGateway_GET(t *testing.T) {
	ts := newTestSession(t, false)
	is := is.New(t)
	resp, err := ts.hc.Get("/ipfs/" + ts.key.B58String() + "/about")
	is.Nil(err)
	is.Equal(resp.StatusCode, http.StatusOK)
}

func TestGateway_POSTwDisabled(t *testing.T) {
	ts := newTestSession(t, false)
	is := is.New(t)
	resp, err := ts.hc.Post("/ipfs/"+ts.key.B58String()+"/new", "test", nil)
	is.Nil(err)
	is.Equal(resp.StatusCode, http.StatusMethodNotAllowed)
}

func TestGateway_Meaningful(t *testing.T) {
	ts := newTestSession(t, true)
	tcases := []struct {
		Method, URL string
		StatusCode  int
		Body        []byte
		Location    string
	}{
		// POST /ipfs creates a new resource under /ipfs
		// whose name (in this case: hash) is determined by the gateway and receives the response:
		{"POST", "/ipfs", http.StatusCreated, []byte("Hello World"), "/ipfs/QmUXTtySmd7LD4p6RG6rZW6RuUuPZXTtNMmRQ6DSQo3aMw"},

		// TODO(cryptix): figure out how to specify the file/link name
		{"POST", "/ipfs/" + ts.key.B58String(), http.StatusCreated, []byte("Hello World"), "/ipfs/QmUXTtySmd7LD4p6RG6rZW6RuUuPZXTtNMmRQ6DSQo3aMw"},

		{"DELETE", "/ipfs/" + ts.key.B58String() + "/about", http.StatusCreated, []byte{}, "/ipfs/test"},
	}
	for i, tcase := range tcases {
		req, err := http.NewRequest(tcase.Method, tcase.URL, bytes.NewReader(tcase.Body))
		if err != nil {
			t.Errorf("case %d NewRequest() failed: %s", i, err)
		}
		resp, err := ts.hc.Do(req)
		if err != nil {
			t.Errorf("case %d failed with error: %s", i, err)
		}
		if resp.StatusCode != tcase.StatusCode {
			t.Errorf("case %d: status mismatch: want: %3d. got: %3d", i, tcase.StatusCode, resp.StatusCode)
			if b, err := ioutil.ReadAll(resp.Body); err == nil && len(b) > 0 {
				t.Logf("response body: %q", b)
			}
		}

		if got := resp.Header.Get("Location"); got != tcase.Location {
			t.Errorf("case %d: location mismatch: want: %s. got: %s", i, tcase.Location, got)
		}
	}
}

func TestGateway_NonMeaningful(t *testing.T) {
	ts := newTestSession(t, true)
	tcases := []struct {
		Method, URL string
		StatusCode  int
	}{
		// PUT /ipfs is not meaningful (“I expect a future GET /ipfs to return this content”).
		{"PUT", "/ipfs", http.StatusMethodNotAllowed},

		// PUT /ipfs/QmFoo can only succeed if QmFoo is in fact the hash of the object being uploaded
		// (requiring the client to compute the hash in advance).
		{"PUT", "/ipfs/" + ts.key.B58String(), http.StatusMethodNotAllowed},

		// PUT /ipfs/QmBar/baz is not meaningful.
		// the gateway might not know anything about QmBar under which baz is requested to be created,
		// and if it does know, QmBar already exists and is immutable.
		{"PUT", "/ipfs/" + ts.key.B58String() + "/about", http.StatusMethodNotAllowed},

		// DELETE /ipfs is not meaningful.
		{"DELETE", "/ipfs", http.StatusMethodNotAllowed},

		// DELETE /ipfs/QmFoo is not meaningful.
		{"DELETE", "/ipfs/" + ts.key.B58String(), http.StatusMethodNotAllowed},
	}

	for i, tcase := range tcases {
		req, err := http.NewRequest(tcase.Method, tcase.URL, bytes.NewReader([]byte{}))
		if err != nil {
			t.Errorf("case %d NewRequest() failed: %s", i, err)
		}
		resp, err := ts.hc.Do(req)
		if err != nil {
			t.Errorf("case %d failed with error: %s", i, err)
		}
		if resp.StatusCode != tcase.StatusCode {
			t.Errorf("case %d: status mismatch: want: %3d. got: %3d", i, tcase.StatusCode, resp.StatusCode)
			if b, err := ioutil.ReadAll(resp.Body); err == nil && len(b) > 0 {
				t.Logf("response body: %q", b)
			}
		}
	}
}

// muxTransport serves http requests without networking
type muxTransport http.ServeMux

func (t *muxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rw := httptest.NewRecorder()
	rw.Body = new(bytes.Buffer)
	(*http.ServeMux)(t).ServeHTTP(rw, req)
	return &http.Response{
		StatusCode:    rw.Code,
		Status:        http.StatusText(rw.Code),
		Header:        rw.HeaderMap,
		Body:          ioutil.NopCloser(rw.Body),
		ContentLength: int64(rw.Body.Len()),
		Request:       req,
	}, nil
}
