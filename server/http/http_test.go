package http

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

type test struct {
	url      string
	code     int
	reqbody  string
	respbody string
}

func TestServeHTTP(t *testing.T) {
	testhandler := &handler{&testIpfsHandler{}}
	tests := []test{
		{"/", http.StatusInternalServerError, "", ""},
		{"/hash", http.StatusOK, "", "some fine data"},
		{"/hash2", http.StatusInternalServerError, "", ""},
	}

	for _, test := range tests {
		req, _ := http.NewRequest("GET", test.url, nil)
		resp := httptest.NewRecorder()
		testhandler.ServeHTTP(resp, req)

		if resp.Code != test.code {
			t.Error("expected status code", test.code, "received", resp.Code)
		}

		if resp.Body.String() != test.respbody {
			t.Error("expected body:", test.respbody)
			t.Error("received body:", resp.Body)
		}
	}
}

func TestPostHandler(t *testing.T) {
	testhandler := &handler{&testIpfsHandler{}}
	tests := []test{
		{"/", http.StatusInternalServerError, "", ""},
		{"/", http.StatusInternalServerError, "something that causes an error in adding to DAG", ""},
		{"/", http.StatusCreated, "some fine data", "jSQBpNSebeYbPBjs1vp"},
	}

	for _, test := range tests {
		req, _ := http.NewRequest("POST", test.url, strings.NewReader(test.reqbody))
		resp := httptest.NewRecorder()
		testhandler.postHandler(resp, req)

		if resp.Code != test.code {
			t.Error("expected status code", test.code, "received", resp.Code)
		}

		if resp.Body.String() != test.respbody {
			t.Error("expected body:", test.respbody)
			t.Error("received body:", resp.Body)
		}
	}
}

type testIpfsHandler struct{}

func (i *testIpfsHandler) ResolvePath(path string) (*dag.Node, error) {
	if path == "/hash" {
		return &dag.Node{Data: []byte("some fine data")}, nil
	}

	if path == "/hash2" {
		return &dag.Node{Data: []byte("data that breaks dagreader")}, nil
	}

	return nil, errors.New("")
}

func (i *testIpfsHandler) NewDagFromReader(r io.Reader) (*dag.Node, error) {
	if data, err := ioutil.ReadAll(r); err == nil {
		return &dag.Node{Data: data}, nil
	}

	return nil, errors.New("")
}

func (i *testIpfsHandler) AddNodeToDAG(nd *dag.Node) (u.Key, error) {
	if len(nd.Data) != 0 && string(nd.Data) != "something that causes an error in adding to DAG" {
		return u.Key(nd.Data), nil
	}

	return "", errors.New("")
}

func (i *testIpfsHandler) NewDagReader(nd *dag.Node) (io.Reader, error) {
	if string(nd.Data) != "data that breaks dagreader" {
		return bytes.NewReader(nd.Data), nil
	}

	return nil, errors.New("")
}
