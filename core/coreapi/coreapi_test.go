package coreapi_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	// coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	testutil "github.com/ipfs/go-ipfs/thirdparty/testutil"
)

// `ipfs object new unixfs-dir`
var emptyUnixfsDir = "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"

// echo -n | ipfs add
// curl -X POST localhost:8080/ipfs/
var emptyUnixfsFile = "QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH"

func makeAPI(ctx context.Context) (*core.IpfsNode, *coreapi.UnixfsAPI, error) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: "Qmfoo", // required by offline node
			},
		},
		D: testutil.ThreadSafeCloserMapDatastore(),
	}
	node, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return nil, nil, err
	}
	api := &coreapi.UnixfsAPI{Node: node, Context: ctx}
	return node, api, nil
}

func testAddBasic(t *testing.T) {
	t.Skip("TODO: implement me")
}

func TestAddEmpty(t *testing.T) {
	t.Skip("TODO: implement me")
}

func TestCatBasic(t *testing.T) {
	node, api, err := makeAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	hello := "hello, world!"
	hr := strings.NewReader(hello)
	k, err := coreunix.Add(node, hr)
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Cat(k)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, len(hello))
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF {
		t.Error(err)
	}
	if string(buf) != hello {
		t.Fatalf("expected [hello, world!], got [%s] [err=%s]", string(buf), n, err)
	}
}

func TestCatEmptyFile(t *testing.T) {
	node, api, err := makeAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	_, err = coreunix.Add(node, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Cat(emptyUnixfsFile)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1) // non-zero so that Read() actually tries to read
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF {
		t.Error(err)
	}
	if !bytes.HasPrefix(buf, []byte{0x00}) {
		t.Fatalf("expected empty data, got [%s] [read=%d]", buf, n)
	}
}

func TestCatDir(t *testing.T) {
	t.Skip("TODO: implement me")
}

func TestCatNonUnixfs(t *testing.T) {
	t.Skip("TODO: implement me")
}

func TestCatOffline(t *testing.T) {
	t.Skip("TODO: implement me")
}

func TestLs(t *testing.T) {
	t.Skip("TODO: implement me")
}

func TestLsEmpty(t *testing.T) {
	t.Skip("TODO: implement me")
}

func TestLsNonUnixfs(t *testing.T) {
	t.Skip("TODO: implement me")
}
