package coreapi_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	"github.com/ipfs/go-ipfs/core/coreunix"
	mock "github.com/ipfs/go-ipfs/core/mock"
	"github.com/ipfs/go-ipfs/keystore"
	"github.com/ipfs/go-ipfs/repo"

	ci "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	pstore "gx/ipfs/QmQAGG1zxfePqj2t7bLxyN8AFccZ889DDR9Gn8kVLDrGZo/go-libp2p-peerstore"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"
	unixfs "gx/ipfs/QmUnHNqhSB1JgzVCxL1Kz3yb4bdyB4q1Z9AD5AUBVmt3fZ/go-unixfs"
	mocknet "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p/p2p/net/mock"
	files "gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"
	config "gx/ipfs/QmbK4EmM2Xx5fmbqK38TGP3PpY66r3tkXLZTcc7dF9mFwM/go-ipfs-config"
	mdag "gx/ipfs/QmcGt25mrjuB2kKW2zhPbXVZNHc4yoTDQ65NA8m6auP2f1/go-merkledag"
	peer "gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"
	datastore "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
	syncds "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore/sync"
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
			ExtraOpts: map[string]bool{
				"pubsub": true,
			},
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

func twoLevelDir() func() files.File {
	return func() files.File {
		return files.NewSliceFile("t", "t", []files.File{
			files.NewSliceFile("t/abc", "t/abc", []files.File{
				files.NewReaderFile("t/abc/def", "t/abc/def", ioutil.NopCloser(strings.NewReader("world")), nil),
			}),
			files.NewReaderFile("t/bar", "t/bar", ioutil.NopCloser(strings.NewReader("hello2")), nil),
			files.NewReaderFile("t/foo", "t/foo", ioutil.NopCloser(strings.NewReader("hello1")), nil),
		})
	}
}

func flatDir() files.File {
	return files.NewSliceFile("t", "t", []files.File{
		files.NewReaderFile("t/bar", "t/bar", ioutil.NopCloser(strings.NewReader("hello2")), nil),
		files.NewReaderFile("t/foo", "t/foo", ioutil.NopCloser(strings.NewReader("hello1")), nil),
	})
}

func wrapped(f files.File) files.File {
	return files.NewSliceFile("", "", []files.File{
		f,
	})
}

func TestAdd(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	cases := []struct {
		name   string
		data   func() files.File
		expect func(files.File) files.File

		path string
		err  string

		recursive bool

		events []coreiface.AddEvent

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
		{
			name: "hashOnly", // test (non)fetchability
			data: strFile(helloStr),
			path: hello,
			opts: []options.UnixfsAddOption{options.Unixfs.HashOnly(true)},
		},
		// multi file
		{
			name:      "simpleDir",
			data:      flatDir,
			recursive: true,
			path:      "/ipfs/QmRKGpFfR32FVXdvJiHfo4WJ5TDYBsM1P9raAp1p6APWSp",
		},
		{
			name:      "twoLevelDir",
			data:      twoLevelDir(),
			recursive: true,
			path:      "/ipfs/QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr",
		},
		// wrapped
		{
			name: "addWrapped",
			path: "/ipfs/QmVE9rNpj5doj7XHzp5zMUxD7BJgXEqx4pe3xZ3JBReWHE",
			data: func() files.File {
				return files.NewReaderFile("foo", "foo", ioutil.NopCloser(strings.NewReader(helloStr)), nil)
			},
			expect: wrapped,
			opts:   []options.UnixfsAddOption{options.Unixfs.Wrap(true)},
		},
		{
			name: "stdinWrapped",
			path: "/ipfs/QmU3r81oZycjHS9oaSHw37ootMFuFUw1DvMLKXPsezdtqU",
			data: func() files.File {
				return files.NewReaderFile("", os.Stdin.Name(), ioutil.NopCloser(strings.NewReader(helloStr)), nil)
			},
			expect: func(files.File) files.File {
				return files.NewSliceFile("", "", []files.File{
					files.NewReaderFile("QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk", "QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk", ioutil.NopCloser(strings.NewReader(helloStr)), nil),
				})
			},
			opts: []options.UnixfsAddOption{options.Unixfs.Wrap(true)},
		},
		{
			name: "stdinNamed",
			path: "/ipfs/QmQ6cGBmb3ZbdrQW1MRm1RJnYnaxCqfssz7CrTa9NEhQyS",
			data: func() files.File {
				return files.NewReaderFile("", os.Stdin.Name(), ioutil.NopCloser(strings.NewReader(helloStr)), nil)
			},
			expect: func(files.File) files.File {
				return files.NewSliceFile("", "", []files.File{
					files.NewReaderFile("test", "test", ioutil.NopCloser(strings.NewReader(helloStr)), nil),
				})
			},
			opts: []options.UnixfsAddOption{options.Unixfs.Wrap(true), options.Unixfs.StdinName("test")},
		},
		{
			name:      "twoLevelDirWrapped",
			data:      twoLevelDir(),
			recursive: true,
			expect:    wrapped,
			path:      "/ipfs/QmPwsL3T5sWhDmmAWZHAzyjKtMVDS9a11aHNRqb3xoVnmg",
			opts:      []options.UnixfsAddOption{options.Unixfs.Wrap(true)},
		},
		{
			name:      "twoLevelInlineHash",
			data:      twoLevelDir(),
			recursive: true,
			expect:    wrapped,
			path:      "/ipfs/zBunoruKoyCHKkALNSWxDvj4L7yuQnMgQ4hUa9j1Z64tVcDEcu6Zdetyu7eeFCxMPfxb7YJvHeFHoFoHMkBUQf6vfdhmi",
			opts:      []options.UnixfsAddOption{options.Unixfs.Wrap(true), options.Unixfs.Inline(true), options.Unixfs.RawLeaves(true), options.Unixfs.Hash(mh.SHA3)},
		},
		// hidden
		{
			name: "hiddenFiles",
			data: func() files.File {
				return files.NewSliceFile("t", "t", []files.File{
					files.NewReaderFile("t/.bar", "t/.bar", ioutil.NopCloser(strings.NewReader("hello2")), nil),
					files.NewReaderFile("t/bar", "t/bar", ioutil.NopCloser(strings.NewReader("hello2")), nil),
					files.NewReaderFile("t/foo", "t/foo", ioutil.NopCloser(strings.NewReader("hello1")), nil),
				})
			},
			recursive: true,
			path:      "/ipfs/QmehGvpf2hY196MzDFmjL8Wy27S4jbgGDUAhBJyvXAwr3g",
			opts:      []options.UnixfsAddOption{options.Unixfs.Hidden(true)},
		},
		{
			name: "hiddenFileAlwaysAdded",
			data: func() files.File {
				return files.NewReaderFile(".foo", ".foo", ioutil.NopCloser(strings.NewReader(helloStr)), nil)
			},
			recursive: true,
			path:      hello,
		},
		{
			name: "hiddenFilesNotAdded",
			data: func() files.File {
				return files.NewSliceFile("t", "t", []files.File{
					files.NewReaderFile("t/.bar", "t/.bar", ioutil.NopCloser(strings.NewReader("hello2")), nil),
					files.NewReaderFile("t/bar", "t/bar", ioutil.NopCloser(strings.NewReader("hello2")), nil),
					files.NewReaderFile("t/foo", "t/foo", ioutil.NopCloser(strings.NewReader("hello1")), nil),
				})
			},
			expect: func(files.File) files.File {
				return flatDir()
			},
			recursive: true,
			path:      "/ipfs/QmRKGpFfR32FVXdvJiHfo4WJ5TDYBsM1P9raAp1p6APWSp",
			opts:      []options.UnixfsAddOption{options.Unixfs.Hidden(false)},
		},
		// Events / Progress
		{
			name: "simpleAddEvent",
			data: strFile(helloStr),
			path: "/ipfs/zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd",
			events: []coreiface.AddEvent{
				{Name: "zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd", Hash: "zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd", Size: strconv.Itoa(len(helloStr))},
			},
			opts: []options.UnixfsAddOption{options.Unixfs.RawLeaves(true)},
		},
		{
			name: "silentAddEvent",
			data: twoLevelDir(),
			path: "/ipfs/QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr",
			events: []coreiface.AddEvent{
				{Name: "t/abc", Hash: "QmU7nuGs2djqK99UNsNgEPGh6GV4662p6WtsgccBNGTDxt", Size: "62"},
				{Name: "t", Hash: "QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr", Size: "229"},
			},
			recursive: true,
			opts:      []options.UnixfsAddOption{options.Unixfs.Silent(true)},
		},
		{
			name: "dirAddEvents",
			data: twoLevelDir(),
			path: "/ipfs/QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr",
			events: []coreiface.AddEvent{
				{Name: "t/abc/def", Hash: "QmNyJpQkU1cEkBwMDhDNFstr42q55mqG5GE5Mgwug4xyGk", Size: "13"},
				{Name: "t/bar", Hash: "QmS21GuXiRMvJKHos4ZkEmQDmRBqRaF5tQS2CQCu2ne9sY", Size: "14"},
				{Name: "t/foo", Hash: "QmfAjGiVpTN56TXi6SBQtstit5BEw3sijKj1Qkxn6EXKzJ", Size: "14"},
				{Name: "t/abc", Hash: "QmU7nuGs2djqK99UNsNgEPGh6GV4662p6WtsgccBNGTDxt", Size: "62"},
				{Name: "t", Hash: "QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr", Size: "229"},
			},
			recursive: true,
		},
		{
			name: "progress1M",
			data: func() files.File {
				r := bytes.NewReader(bytes.Repeat([]byte{0}, 1000000))
				return files.NewReaderFile("", "", ioutil.NopCloser(r), nil)
			},
			path: "/ipfs/QmXXNNbwe4zzpdMg62ZXvnX1oU7MwSrQ3vAEtuwFKCm1oD",
			events: []coreiface.AddEvent{
				{Name: "", Bytes: 262144},
				{Name: "", Bytes: 524288},
				{Name: "", Bytes: 786432},
				{Name: "", Bytes: 1000000},
				{Name: "QmXXNNbwe4zzpdMg62ZXvnX1oU7MwSrQ3vAEtuwFKCm1oD", Hash: "QmXXNNbwe4zzpdMg62ZXvnX1oU7MwSrQ3vAEtuwFKCm1oD", Size: "1000256"},
			},
			recursive: true,
			opts:      []options.UnixfsAddOption{options.Unixfs.Progress(true)},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			// recursive logic

			data := testCase.data()
			if testCase.recursive {
				data = files.NewSliceFile("", "", []files.File{
					data,
				})
			}

			// handle events if relevant to test case

			opts := testCase.opts
			eventOut := make(chan interface{})
			var evtWg sync.WaitGroup
			if len(testCase.events) > 0 {
				opts = append(opts, options.Unixfs.Events(eventOut))
				evtWg.Add(1)

				go func() {
					defer evtWg.Done()
					expected := testCase.events

					for evt := range eventOut {
						event, ok := evt.(*coreiface.AddEvent)
						if !ok {
							t.Fatal("unexpected event type")
						}

						if len(expected) < 1 {
							t.Fatal("got more events than expected")
						}

						if expected[0].Size != event.Size {
							t.Errorf("Event.Size didn't match, %s != %s", expected[0].Size, event.Size)
						}

						if expected[0].Name != event.Name {
							t.Errorf("Event.Name didn't match, %s != %s", expected[0].Name, event.Name)
						}

						if expected[0].Hash != event.Hash {
							t.Errorf("Event.Hash didn't match, %s != %s", expected[0].Hash, event.Hash)
						}
						if expected[0].Bytes != event.Bytes {
							t.Errorf("Event.Bytes didn't match, %d != %d", expected[0].Bytes, event.Bytes)
						}

						expected = expected[1:]
					}

					if len(expected) > 0 {
						t.Fatalf("%d event(s) didn't arrive", len(expected))
					}
				}()
			}

			// Add!

			p, err := api.Unixfs().Add(ctx, data, opts...)
			close(eventOut)
			evtWg.Wait()
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
				t.Fatal(err)
			}

			if p.String() != testCase.path {
				t.Errorf("expected path %s, got: %s", testCase.path, p)
			}

			// compare file structure with Unixfs().Get

			var cmpFile func(orig files.File, got files.File)
			cmpFile = func(orig files.File, got files.File) {
				if orig.IsDirectory() != got.IsDirectory() {
					t.Fatal("file type mismatch")
				}

				if !orig.IsDirectory() {
					defer orig.Close()
					defer got.Close()

					do, err := ioutil.ReadAll(orig)
					if err != nil {
						t.Fatal(err)
					}

					dg, err := ioutil.ReadAll(got)
					if err != nil {
						t.Fatal(err)
					}

					if !bytes.Equal(do, dg) {
						t.Fatal("data not equal")
					}

					return
				}

				for {
					fo, err := orig.NextFile()
					fg, err2 := got.NextFile()

					if err != nil {
						if err == io.EOF && err2 == io.EOF {
							break
						}
						t.Fatal(err)
					}
					if err2 != nil {
						t.Fatal(err)
					}

					cmpFile(fo, fg)
				}
			}

			f, err := api.Unixfs().Get(ctx, p)
			if err != nil {
				t.Fatal(err)
			}

			orig := testCase.data()
			if testCase.expect != nil {
				orig = testCase.expect(orig)
			}

			cmpFile(orig, f)
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

func TestGetEmptyFile(t *testing.T) {
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

	r, err := api.Unixfs().Get(ctx, emptyFilePath)
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

func TestGetDir(t *testing.T) {
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

	r, err := api.Unixfs().Get(ctx, coreiface.IpfsPath(emptyDir.Cid()))
	if err != nil {
		t.Error(err)
	}

	_, err = r.Read(make([]byte, 2))
	if err != files.ErrNotReader {
		t.Fatalf("expected ErrIsDir, got: %s", err)
	}
}

func TestGetNonUnixfs(t *testing.T) {
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

	_, err = api.Unixfs().Get(ctx, coreiface.IpfsPath(nd.Cid()))
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
	_, err = api.Unixfs().Get(ctx, p)
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
