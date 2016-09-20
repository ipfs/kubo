package coreapi_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	testutil "github.com/ipfs/go-ipfs/thirdparty/testutil"
	unixfs "github.com/ipfs/go-ipfs/unixfs"
)

// `echo -n 'hello, world!' | ipfs add`
var hello = "QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk"
var helloStr = "hello, world!"

// `ipfs object new unixfs-dir`
var emptyUnixfsDir = "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"

// `echo -n | ipfs add`
var emptyUnixfsFile = "QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH"

func makeAPI(ctx context.Context) (*core.IpfsNode, coreiface.UnixfsAPI, error) {
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
	api := coreapi.NewUnixfsAPI(node)
	return node, api, nil
}

func TestAdd(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	str := strings.NewReader(helloStr)
	c, err := api.Add(ctx, str)
	if err != nil {
		t.Error(err)
	}

	if c.String() != hello {
		t.Fatalf("expected CID %s, got: %s", hello, c)
	}

	r, err := api.Cat(ctx, hello)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(helloStr))
	_, err = io.ReadFull(r, buf)
	if err != nil {
		t.Error(err)
	}

	if string(buf) != helloStr {
		t.Fatalf("expected [%s], got [%s] [err=%s]", helloStr, string(buf), err)
	}
}

func TestAddEmptyFile(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	str := strings.NewReader("")
	c, err := api.Add(ctx, str)
	if err != nil {
		t.Error(err)
	}

	if c.String() != emptyUnixfsFile {
		t.Fatalf("expected CID %s, got: %s", hello, c)
	}
}

func TestCatBasic(t *testing.T) {
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	hr := strings.NewReader(helloStr)
	k, err := coreunix.Add(node, hr)
	if err != nil {
		t.Fatal(err)
	}

	if k != hello {
		t.Fatalf("expected CID %s, got: %s", hello, k)
	}

	r, err := api.Cat(ctx, k)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, len(helloStr))
	_, err = io.ReadFull(r, buf)
	if err != nil {
		t.Error(err)
	}
	if string(buf) != helloStr {
		t.Fatalf("expected [%s], got [%s] [err=%s]", helloStr, string(buf), err)
	}
}

func TestCatEmptyFile(t *testing.T) {
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = coreunix.Add(node, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Cat(ctx, emptyUnixfsFile)
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
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	c, err := node.DAG.Add(unixfs.EmptyDirNode())
	if err != nil {
		t.Error(err)
	}

	_, err = api.Cat(ctx, c.String())
	if err != coreiface.ErrIsDir {
		t.Fatalf("expected ErrIsDir, got: %s", err)
	}
}

func TestCatNonUnixfs(t *testing.T) {
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	c, err := node.DAG.Add(new(mdag.ProtoNode))
	if err != nil {
		t.Error(err)
	}

	_, err = api.Cat(ctx, c.String())
	if !strings.Contains(err.Error(), "proto: required field") {
		t.Fatalf("expected protobuf error, got: %s", err)
	}
}

func TestCatOffline(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Cat(ctx, "/ipns/Qmfoobar")
	if err != coreiface.ErrOffline {
		t.Fatalf("expected ErrOffline, got: %", err)
	}
}

func TestLs(t *testing.T) {
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	r := strings.NewReader("content-of-file")
	p, _, err := coreunix.AddWrapped(node, r, "name-of-file")
	if err != nil {
		t.Error(err)
	}
	parts := strings.Split(p, "/")
	if len(parts) != 2 {
		t.Errorf("unexpected path:", p)
	}
	k := parts[0]

	links, err := api.Ls(ctx, k)
	if err != nil {
		t.Error(err)
	}

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Size != 23 {
		t.Fatalf("expected size = 23, got %d", links[0].Size)
	}
	if links[0].Name != "name-of-file" {
		t.Fatalf("expected name = name-of-file, got %s", links[0].Name)
	}
	if links[0].Cid.String() != "QmX3qQVKxDGz3URVC3861Z3CKtQKGBn6ffXRBBWGMFz9Lr" {
		t.Fatalf("expected cid = QmX3qQVKxDGz3URVC3861Z3CKtQKGBn6ffXRBBWGMFz9Lr, got %s", links[0].Cid.String())
	}
}

func TestLsEmptyDir(t *testing.T) {
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	c, err := node.DAG.Add(unixfs.EmptyDirNode())
	if err != nil {
		t.Error(err)
	}

	links, err := api.Ls(ctx, c.String())
	if err != nil {
		t.Error(err)
	}

	if len(links) != 0 {
		t.Fatalf("expected 0 links, got %d", len(links))
	}
}

// TODO(lgierth) this should test properly, with len(links) > 0
func TestLsNonUnixfs(t *testing.T) {
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	c, err := node.DAG.Add(new(mdag.ProtoNode))
	if err != nil {
		t.Error(err)
	}

	links, err := api.Ls(ctx, c.String())
	if err != nil {
		t.Error(err)
	}

	if len(links) != 0 {
		t.Fatalf("expected 0 links, got %d", len(links))
	}
}
