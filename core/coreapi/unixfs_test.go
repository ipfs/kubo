package coreapi_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"strings"
	"testing"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	"github.com/ipfs/go-ipfs/core/coreunix"
	mock "github.com/ipfs/go-ipfs/core/mock"
	"github.com/ipfs/go-ipfs/keystore"
	"github.com/ipfs/go-ipfs/repo"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	files "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit/files"
	cbor "gx/ipfs/QmSywXfm2v4Qkp4DcFqo8eehj49dJK3bdUnaLVxrdFLMQn/go-ipld-cbor"
	unixfs "gx/ipfs/QmU4x3742bvgfxJsByEDpBnifJqjJdV6x528co4hwKCn46/go-unixfs"
	datastore "gx/ipfs/QmUyz7JTJzgegC6tiJrfby3mPhzcdswVtG4x58TQ6pq8jV/go-datastore"
	syncds "gx/ipfs/QmUyz7JTJzgegC6tiJrfby3mPhzcdswVtG4x58TQ6pq8jV/go-datastore/sync"
	config "gx/ipfs/QmVBUpxsHh53rNcufqxMpLAmz37eGyLJUaexDy1W9YkiNk/go-ipfs-config"
	mocknet "gx/ipfs/QmVsVARb86uSe1qYouewFMNd2p2sp2NGWm1JGPReVDWchW/go-libp2p/p2p/net/mock"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
	mdag "gx/ipfs/QmcBoNcAP6qDjgRBew7yjvCqHq7p5jMstE44jPUBWBxzsV/go-merkledag"
	pstore "gx/ipfs/QmfAQMFpgDU2U4BXG64qVr8HSiictfWvkSBz7Y2oDj65st/go-libp2p-peerstore"
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

func strFile(data string) func() files.File {
	return func() files.File {
		return files.NewReaderFile("", "", ioutil.NopCloser(strings.NewReader(data)), nil)
	}
}

func TestAdd(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	cases := []struct {
		name string
		data func() files.File
		path string
		err  string
		opts []options.UnixfsAddOption
	}{
		// Simple cases
		{
			name: "simpleAdd",
			data: strFile(helloStr),
			path: hello,
			opts: []options.UnixfsAddOption{},
		},
		{
			name: "addEmpty",
			data: strFile(""),
			path: emptyFile,
		},
		// CIDv1 version / rawLeaves
		{
			name: "addCidV1",
			data: strFile(helloStr),
			path: "/ipfs/zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd",
			opts: []options.UnixfsAddOption{options.Unixfs.CidVersion(1)},
		},
		{
			name: "addCidV1NoLeaves",
			data: strFile(helloStr),
			path: "/ipfs/zdj7WY4GbN8NDbTW1dfCShAQNVovams2xhq9hVCx5vXcjvT8g",
			opts: []options.UnixfsAddOption{options.Unixfs.CidVersion(1), options.Unixfs.RawLeaves(false)},
		},
		// Non sha256 hash vs CID
		{
			name: "addCidSha3",
			data: strFile(helloStr),
			path: "/ipfs/zb2wwnYtXBxpndNABjtYxWAPt3cwWNRnc11iT63fvkYV78iRb",
			opts: []options.UnixfsAddOption{options.Unixfs.Hash(mh.SHA3_256)},
		},
		{
			name: "addCidSha3Cid0",
			data: strFile(helloStr),
			err:  "CIDv0 only supports sha2-256",
			opts: []options.UnixfsAddOption{options.Unixfs.CidVersion(0), options.Unixfs.Hash(mh.SHA3_256)},
		},
		// Inline
		{
			name: "addInline",
			data: strFile(helloStr),
			path: "/ipfs/zaYomJdLndMku8P9LHngHB5w2CQ7NenLbv",
			opts: []options.UnixfsAddOption{options.Unixfs.Inline(true)},
		},
		{
			name: "addInlineLimit",
			data: strFile(helloStr),
			path: "/ipfs/zaYomJdLndMku8P9LHngHB5w2CQ7NenLbv",
			opts: []options.UnixfsAddOption{options.Unixfs.InlineLimit(32), options.Unixfs.Inline(true)},
		},
		{
			name: "addInlineZero",
			data: strFile(""),
			path: "/ipfs/z2yYDV",
			opts: []options.UnixfsAddOption{options.Unixfs.InlineLimit(0), options.Unixfs.Inline(true), options.Unixfs.RawLeaves(true)},
		},
		{ //TODO: after coreapi add is used in `ipfs add`, consider making this default for inline
			name: "addInlineRaw",
			data: strFile(helloStr),
			path: "/ipfs/zj7Gr8AcBreqGEfrnR5kPFe",
			opts: []options.UnixfsAddOption{options.Unixfs.InlineLimit(32), options.Unixfs.Inline(true), options.Unixfs.RawLeaves(true)},
		},
		// Chunker / Layout
		{
			name: "addChunks",
			data: strFile(strings.Repeat("aoeuidhtns", 200)),
			path: "/ipfs/QmRo11d4QJrST47aaiGVJYwPhoNA4ihRpJ5WaxBWjWDwbX",
			opts: []options.UnixfsAddOption{options.Unixfs.Chunker("size-4")},
		},
		{
			name: "addChunksTrickle",
			data: strFile(strings.Repeat("aoeuidhtns", 200)),
			path: "/ipfs/QmNNhDGttafX3M1wKWixGre6PrLFGjnoPEDXjBYpTv93HP",
			opts: []options.UnixfsAddOption{options.Unixfs.Chunker("size-4"), options.Unixfs.Layout(options.TrickleLayout)},
		},
		// Local
		{
			name: "addLocal", // better cases in sharness
			data: strFile(helloStr),
			path: hello,
			opts: []options.UnixfsAddOption{options.Unixfs.Local(true)},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			p, err := api.Unixfs().Add(ctx, testCase.data(), testCase.opts...)
			if testCase.err != "" {
				if err == nil {
					t.Fatalf("expected an error: %s", testCase.err)
				}
				if err.Error() != testCase.err {
					t.Fatalf("expected an error: '%s' != '%s'", err.Error(), testCase.err)
				}
				return
			}
			if err != nil {
				t.Error(err)
			}

			if p.String() != testCase.path {
				t.Errorf("expected path %s, got: %s", testCase.path, p)
			}

			/*r, err := api.Unixfs().Cat(ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, len(testCase.data))
			_, err = io.ReadFull(r, buf)
			if err != nil {
				t.Error(err)
			}

			if string(buf) != testCase.data {
				t.Fatalf("expected [%s], got [%s] [err=%s]", helloStr, string(buf), err)
			}*/

		})
	}
}

func TestAddPinned(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Unixfs().Add(ctx, strFile(helloStr)(), options.Unixfs.Pin(true))
	if err != nil {
		t.Error(err)
	}

	pins, err := api.Pin().Ls(ctx)
	if len(pins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(pins))
	}

	if pins[0].Path().String() != "/ipld/QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk" {
		t.Fatalf("got unexpected pin: %s", pins[0].Path().String())
	}
}

func TestAddHashOnly(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	p, err := api.Unixfs().Add(ctx, strFile(helloStr)(), options.Unixfs.HashOnly(true))
	if err != nil {
		t.Error(err)
	}

	if p.String() != hello {
		t.Errorf("unxepected path: %s", p.String())
	}

	_, err = api.Block().Get(ctx, p)
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Error() != "blockservice: key not found" {
		t.Errorf("unxepected error: %s", err.Error())
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
