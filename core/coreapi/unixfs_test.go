package coreapi_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	mock "github.com/ipfs/go-ipfs/core/mock"
	keystore "github.com/ipfs/go-ipfs/keystore"
	repo "github.com/ipfs/go-ipfs/repo"

	unixfs "gx/ipfs/QmPL8bYtbACcSFFiSr4s2du7Na382NxRADR8hC7D9FkEA2/go-unixfs"
	cbor "gx/ipfs/QmPrv66vmh2P7vLJMpYx6DWLTNKvVB4Jdkyxs6V3QvWKvf/go-ipld-cbor"
	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	datastore "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore"
	syncds "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore/sync"
	mocknet "gx/ipfs/QmUEqyXr97aUbNmQADHYNknjwjjdVpJXEt1UZXmSG81EV4/go-libp2p/p2p/net/mock"
	mdag "gx/ipfs/QmXv5mwmQ74r4aiHcNeQ4GAmfB3aWJuqaE4WyDfDfvkgLM/go-merkledag"
	config "gx/ipfs/QmYVqYJTVjetcf1guieEgWpK1PZtHPytP624vKzTF1P3r2/go-ipfs-config"
	pstore "gx/ipfs/Qmda4cPRvSRyox3SqgJN6DfSZGU5TtHufPTp9uXjFj71X6/go-libp2p-peerstore"
)

const testPeerID = "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe"

// `echo -n 'hello, world!' | ipfs add`
var hello = "/ipfs/QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk"
var helloStr = "hello, world!"

// `echo -n | ipfs add`
var emptyFile = "/ipfs/QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH"

func makeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]*core.IpfsNode, []coreiface.CoreAPI, error) {
	mn := mocknet.New(ctx)

	nodes := make([]*core.IpfsNode, n)
	apis := make([]coreiface.CoreAPI, n)

	for i := 0; i < n; i++ {
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

		c := config.Config{}
		c.Addresses.Swarm = []string{fmt.Sprintf("/ip4/127.0.%d.1/tcp/4001", i)}
		c.Identity = ident

		r := &repo.Mock{
			C: c,
			D: syncds.MutexWrap(datastore.NewMapDatastore()),
			K: keystore.NewMemKeystore(),
		}

		node, err := core.NewNode(ctx, &core.BuildCfg{
			Repo:   r,
			Host:   mock.MockHostOption(mn),
			Online: fullIdentity,
		})
		if err != nil {
			return nil, nil, err
		}
		nodes[i] = node
		apis[i] = coreapi.NewCoreAPI(node)
	}

	err := mn.LinkAll()
	if err != nil {
		return nil, nil, err
	}

	bsinf := core.BootstrapConfigWithPeers(
		[]pstore.PeerInfo{
			nodes[0].Peerstore.PeerInfo(nodes[0].Identity),
		},
	)

	for _, n := range nodes[1:] {
		if err := n.Bootstrap(bsinf); err != nil {
			return nil, nil, err
		}
	}

	return nodes, apis, nil
}

func makeAPI(ctx context.Context) (*core.IpfsNode, coreiface.CoreAPI, error) {
	nd, api, err := makeAPISwarm(ctx, false, 1)
	if err != nil {
		return nil, nil, err
	}

	return nd[0], api[0], nil
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
