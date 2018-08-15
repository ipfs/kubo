package coreapi_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"math"
	"strings"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	keystore "github.com/ipfs/go-ipfs/keystore"
	repo "github.com/ipfs/go-ipfs/repo"

	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	config "gx/ipfs/QmQSG7YCizeUH2bWatzp6uK9Vm3m7LA5jpxGa9QqgpNKw4/go-ipfs-config"
	mdag "gx/ipfs/QmQzSpSjkdGHW6WFBhUG6P3t9K8yv7iucucT1cQaqJ6tgd/go-merkledag"
	datastore "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	syncds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore/sync"
	cbor "gx/ipfs/QmVhWKoxHMJNbTMEPhqLAjKg1Y65j9tvWNecYWAHwyguAZ/go-ipld-cbor"
	unixfs "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs"
	peer "gx/ipfs/QmcZSzKEM5yDfpZbeEEZaVmaZ1zXm6JWTbrQZSB8hCVPzk/go-libp2p-peer"
)

const testPeerID = "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe"

// `echo -n 'hello, world!' | ipfs add`
var hello = "/ipfs/QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk"
var helloStr = "hello, world!"

// `echo -n | ipfs add`
var emptyFile = "/ipfs/QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH"

func makeAPIIdent(ctx context.Context, fullIdentity bool) (*core.IpfsNode, coreiface.CoreAPI, error) {
	var ident config.Identity
	if fullIdentity {
		sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
		if err != nil {
			return nil, nil, err
		}

		id, err := peer.IDFromPublicKey(pk)
		if err != nil {
			return nil, nil, err
		}

		kbytes, err := sk.Bytes()
		if err != nil {
			return nil, nil, err
		}

		ident = config.Identity{
			PeerID:  id.Pretty(),
			PrivKey: base64.StdEncoding.EncodeToString(kbytes),
		}
	} else {
		ident = config.Identity{
			PeerID: testPeerID,
		}
	}

	r := &repo.Mock{
		C: config.Config{
			Identity: ident,
		},
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
		K: keystore.NewMemKeystore(),
	}
	node, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return nil, nil, err
	}
	api := coreapi.NewCoreAPI(node)
	return node, api, nil
}

func makeAPI(ctx context.Context) (*core.IpfsNode, coreiface.CoreAPI, error) {
	return makeAPIIdent(ctx, false)
}

func TestAdd(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	str := strings.NewReader(helloStr)
	p, err := api.Unixfs().Add(ctx, str)
	if err != nil {
		t.Error(err)
	}

	if p.String() != hello {
		t.Fatalf("expected path %s, got: %s", hello, p)
	}

	r, err := api.Unixfs().Cat(ctx, p)
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
	p, err := api.Unixfs().Add(ctx, str)
	if err != nil {
		t.Error(err)
	}

	if p.String() != emptyFile {
		t.Fatalf("expected path %s, got: %s", hello, p)
	}
}

func TestCatBasic(t *testing.T) {
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	hr := strings.NewReader(helloStr)
	p, err := coreunix.Add(node, hr)
	if err != nil {
		t.Fatal(err)
	}
	p = "/ipfs/" + p

	if p != hello {
		t.Fatalf("expected CID %s, got: %s", hello, p)
	}

	helloPath, err := coreiface.ParsePath(hello)
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Unixfs().Cat(ctx, helloPath)
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

	emptyFilePath, err := coreiface.ParsePath(emptyFile)
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Unixfs().Cat(ctx, emptyFilePath)
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
	edir := unixfs.EmptyDirNode()
	err = node.DAG.Add(ctx, edir)
	if err != nil {
		t.Error(err)
	}
	p := coreiface.IpfsPath(edir.Cid())

	emptyDir, err := api.Object().New(ctx, options.Object.Type("unixfs-dir"))
	if err != nil {
		t.Error(err)
	}

	if p.String() != coreiface.IpfsPath(emptyDir.Cid()).String() {
		t.Fatalf("expected path %s, got: %s", emptyDir.Cid(), p.String())
	}

	_, err = api.Unixfs().Cat(ctx, coreiface.IpfsPath(emptyDir.Cid()))
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

	nd := new(mdag.ProtoNode)
	err = node.DAG.Add(ctx, nd)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Unixfs().Cat(ctx, coreiface.IpfsPath(nd.Cid()))
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

	p, err := coreiface.ParsePath("/ipns/Qmfoobar")
	if err != nil {
		t.Error(err)
	}
	_, err = api.Unixfs().Cat(ctx, p)
	if err != coreiface.ErrOffline {
		t.Fatalf("expected ErrOffline, got: %s", err)
	}
}

func TestLs(t *testing.T) {
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	r := strings.NewReader("content-of-file")
	k, _, err := coreunix.AddWrapped(node, r, "name-of-file")
	if err != nil {
		t.Error(err)
	}
	parts := strings.Split(k, "/")
	if len(parts) != 2 {
		t.Errorf("unexpected path: %s", k)
	}
	p, err := coreiface.ParsePath("/ipfs/" + parts[0])
	if err != nil {
		t.Error(err)
	}

	links, err := api.Unixfs().Ls(ctx, p)
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
		t.Fatalf("expected cid = QmX3qQVKxDGz3URVC3861Z3CKtQKGBn6ffXRBBWGMFz9Lr, got %s", links[0].Cid)
	}
}

func TestLsEmptyDir(t *testing.T) {
	ctx := context.Background()
	node, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	err = node.DAG.Add(ctx, unixfs.EmptyDirNode())
	if err != nil {
		t.Error(err)
	}

	emptyDir, err := api.Object().New(ctx, options.Object.Type("unixfs-dir"))
	if err != nil {
		t.Error(err)
	}

	links, err := api.Unixfs().Ls(ctx, coreiface.IpfsPath(emptyDir.Cid()))
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

	nd, err := cbor.WrapObject(map[string]interface{}{"foo": "bar"}, math.MaxUint64, -1)
	if err != nil {
		t.Fatal(err)
	}

	err = node.DAG.Add(ctx, nd)
	if err != nil {
		t.Error(err)
	}

	links, err := api.Unixfs().Ls(ctx, coreiface.IpfsPath(nd.Cid()))
	if err != nil {
		t.Error(err)
	}

	if len(links) != 0 {
		t.Fatalf("expected 0 links, got %d", len(links))
	}
}
