package tests

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-files"
	cbor "github.com/ipfs/go-ipld-cbor"
	mdag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-unixfs"
	"github.com/ipfs/go-unixfs/importer/helpers"
	mh "github.com/multiformats/go-multihash"
)

func (tp *provider) TestUnixfs(t *testing.T) {
	tp.hasApi(t, func(api coreiface.CoreAPI) error {
		if api.Unixfs() == nil {
			return apiNotImplemented
		}
		return nil
	})

	t.Run("TestAdd", tp.TestAdd)
	t.Run("TestAddPinned", tp.TestAddPinned)
	t.Run("TestAddHashOnly", tp.TestAddHashOnly)
	t.Run("TestGetEmptyFile", tp.TestGetEmptyFile)
	t.Run("TestGetDir", tp.TestGetDir)
	t.Run("TestGetNonUnixfs", tp.TestGetNonUnixfs)
	t.Run("TestLs", tp.TestLs)
	t.Run("TestEntriesExpired", tp.TestEntriesExpired)
	t.Run("TestLsEmptyDir", tp.TestLsEmptyDir)
	t.Run("TestLsNonUnixfs", tp.TestLsNonUnixfs)
	t.Run("TestAddCloses", tp.TestAddCloses)
	t.Run("TestGetSeek", tp.TestGetSeek)
}

// `echo -n 'hello, world!' | ipfs add`
var hello = "/ipfs/QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk"
var helloStr = "hello, world!"

// `echo -n | ipfs add`
var emptyFile = "/ipfs/QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH"

func strFile(data string) func() files.Node {
	return func() files.Node {
		return files.NewBytesFile([]byte(data))
	}
}

func twoLevelDir() func() files.Node {
	return func() files.Node {
		return files.NewMapDirectory(map[string]files.Node{
			"abc": files.NewMapDirectory(map[string]files.Node{
				"def": files.NewBytesFile([]byte("world")),
			}),

			"bar": files.NewBytesFile([]byte("hello2")),
			"foo": files.NewBytesFile([]byte("hello1")),
		})
	}
}

func flatDir() files.Node {
	return files.NewMapDirectory(map[string]files.Node{
		"bar": files.NewBytesFile([]byte("hello2")),
		"foo": files.NewBytesFile([]byte("hello1")),
	})
}

func wrapped(name string) func(f files.Node) files.Node {
	return func(f files.Node) files.Node {
		return files.NewMapDirectory(map[string]files.Node{
			name: f,
		})
	}
}

func (tp *provider) TestAdd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	p := func(h string) coreiface.ResolvedPath {
		c, err := cid.Parse(h)
		if err != nil {
			t.Fatal(err)
		}
		return coreiface.IpfsPath(c)
	}

	rf, err := ioutil.TempFile(os.TempDir(), "unixfs-add-real")
	if err != nil {
		t.Fatal(err)
	}
	rfp := rf.Name()

	if _, err := rf.Write([]byte(helloStr)); err != nil {
		t.Fatal(err)
	}

	stat, err := rf.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if err := rf.Close(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(rfp)

	realFile := func() files.Node {
		n, err := files.NewReaderPathFile(rfp, ioutil.NopCloser(strings.NewReader(helloStr)), stat)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	cases := []struct {
		name   string
		data   func() files.Node
		expect func(files.Node) files.Node

		apiOpts []options.ApiOption

		path string
		err  string

		wrap string

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
			name:    "addLocal", // better cases in sharness
			data:    strFile(helloStr),
			path:    hello,
			apiOpts: []options.ApiOption{options.Api.Offline(true)},
		},
		{
			name: "hashOnly", // test (non)fetchability
			data: strFile(helloStr),
			path: hello,
			opts: []options.UnixfsAddOption{options.Unixfs.HashOnly(true)},
		},
		// multi file
		{
			name: "simpleDir",
			data: flatDir,
			wrap: "t",
			path: "/ipfs/QmRKGpFfR32FVXdvJiHfo4WJ5TDYBsM1P9raAp1p6APWSp",
		},
		{
			name: "twoLevelDir",
			data: twoLevelDir(),
			wrap: "t",
			path: "/ipfs/QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr",
		},
		// wrapped
		{
			name: "addWrapped",
			path: "/ipfs/QmVE9rNpj5doj7XHzp5zMUxD7BJgXEqx4pe3xZ3JBReWHE",
			data: func() files.Node {
				return files.NewBytesFile([]byte(helloStr))
			},
			wrap:   "foo",
			expect: wrapped("foo"),
			opts:   []options.UnixfsAddOption{options.Unixfs.Wrap(true)},
		},
		{
			name: "addNotWrappedDirFile",
			path: hello,
			data: func() files.Node {
				return files.NewBytesFile([]byte(helloStr))
			},
			wrap: "foo",
		},
		{
			name: "stdinWrapped",
			path: "/ipfs/QmU3r81oZycjHS9oaSHw37ootMFuFUw1DvMLKXPsezdtqU",
			data: func() files.Node {
				return files.NewBytesFile([]byte(helloStr))
			},
			expect: func(files.Node) files.Node {
				return files.NewMapDirectory(map[string]files.Node{
					"QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk": files.NewBytesFile([]byte(helloStr)),
				})
			},
			opts: []options.UnixfsAddOption{options.Unixfs.Wrap(true)},
		},
		{
			name: "stdinNamed",
			path: "/ipfs/QmQ6cGBmb3ZbdrQW1MRm1RJnYnaxCqfssz7CrTa9NEhQyS",
			data: func() files.Node {
				rf, err := files.NewReaderPathFile(os.Stdin.Name(), ioutil.NopCloser(strings.NewReader(helloStr)), nil)
				if err != nil {
					panic(err)
				}

				return rf
			},
			expect: func(files.Node) files.Node {
				return files.NewMapDirectory(map[string]files.Node{
					"test": files.NewBytesFile([]byte(helloStr)),
				})
			},
			opts: []options.UnixfsAddOption{options.Unixfs.Wrap(true), options.Unixfs.StdinName("test")},
		},
		{
			name:   "twoLevelDirWrapped",
			data:   twoLevelDir(),
			wrap:   "t",
			expect: wrapped("t"),
			path:   "/ipfs/QmPwsL3T5sWhDmmAWZHAzyjKtMVDS9a11aHNRqb3xoVnmg",
			opts:   []options.UnixfsAddOption{options.Unixfs.Wrap(true)},
		},
		{
			name:   "twoLevelInlineHash",
			data:   twoLevelDir(),
			wrap:   "t",
			expect: wrapped("t"),
			path:   "/ipfs/zBunoruKoyCHKkALNSWxDvj4L7yuQnMgQ4hUa9j1Z64tVcDEcu6Zdetyu7eeFCxMPfxb7YJvHeFHoFoHMkBUQf6vfdhmi",
			opts:   []options.UnixfsAddOption{options.Unixfs.Wrap(true), options.Unixfs.Inline(true), options.Unixfs.RawLeaves(true), options.Unixfs.Hash(mh.SHA3)},
		},
		// hidden
		{
			name: "hiddenFiles",
			data: func() files.Node {
				return files.NewMapDirectory(map[string]files.Node{
					".bar": files.NewBytesFile([]byte("hello2")),
					"bar":  files.NewBytesFile([]byte("hello2")),
					"foo":  files.NewBytesFile([]byte("hello1")),
				})
			},
			wrap: "t",
			path: "/ipfs/QmehGvpf2hY196MzDFmjL8Wy27S4jbgGDUAhBJyvXAwr3g",
			opts: []options.UnixfsAddOption{options.Unixfs.Hidden(true)},
		},
		{
			name: "hiddenFileAlwaysAdded",
			data: func() files.Node {
				return files.NewBytesFile([]byte(helloStr))
			},
			wrap: ".foo",
			path: hello,
		},
		{
			name: "hiddenFilesNotAdded",
			data: func() files.Node {
				return files.NewMapDirectory(map[string]files.Node{
					".bar": files.NewBytesFile([]byte("hello2")),
					"bar":  files.NewBytesFile([]byte("hello2")),
					"foo":  files.NewBytesFile([]byte("hello1")),
				})
			},
			expect: func(files.Node) files.Node {
				return flatDir()
			},
			wrap: "t",
			path: "/ipfs/QmRKGpFfR32FVXdvJiHfo4WJ5TDYBsM1P9raAp1p6APWSp",
			opts: []options.UnixfsAddOption{options.Unixfs.Hidden(false)},
		},
		// NoCopy
		{
			name: "simpleNoCopy",
			data: realFile,
			path: "/ipfs/zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd",
			opts: []options.UnixfsAddOption{options.Unixfs.Nocopy(true)},
		},
		{
			name: "noCopyNoRaw",
			data: realFile,
			path: "/ipfs/zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd",
			opts: []options.UnixfsAddOption{options.Unixfs.Nocopy(true), options.Unixfs.RawLeaves(false)},
			err:  "nocopy option requires '--raw-leaves' to be enabled as well",
		},
		{
			name: "noCopyNoPath",
			data: strFile(helloStr),
			path: "/ipfs/zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd",
			opts: []options.UnixfsAddOption{options.Unixfs.Nocopy(true)},
			err:  helpers.ErrMissingFsRef.Error(),
		},
		// Events / Progress
		{
			name: "simpleAddEvent",
			data: strFile(helloStr),
			path: "/ipfs/zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd",
			events: []coreiface.AddEvent{
				{Name: "zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd", Path: p("zb2rhdhmJjJZs9qkhQCpCQ7VREFkqWw3h1r8utjVvQugwHPFd"), Size: strconv.Itoa(len(helloStr))},
			},
			opts: []options.UnixfsAddOption{options.Unixfs.RawLeaves(true)},
		},
		{
			name: "silentAddEvent",
			data: twoLevelDir(),
			path: "/ipfs/QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr",
			events: []coreiface.AddEvent{
				{Name: "t/abc", Path: p("QmU7nuGs2djqK99UNsNgEPGh6GV4662p6WtsgccBNGTDxt"), Size: "62"},
				{Name: "t", Path: p("QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr"), Size: "229"},
			},
			wrap: "t",
			opts: []options.UnixfsAddOption{options.Unixfs.Silent(true)},
		},
		{
			name: "dirAddEvents",
			data: twoLevelDir(),
			path: "/ipfs/QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr",
			events: []coreiface.AddEvent{
				{Name: "t/abc/def", Path: p("QmNyJpQkU1cEkBwMDhDNFstr42q55mqG5GE5Mgwug4xyGk"), Size: "13"},
				{Name: "t/bar", Path: p("QmS21GuXiRMvJKHos4ZkEmQDmRBqRaF5tQS2CQCu2ne9sY"), Size: "14"},
				{Name: "t/foo", Path: p("QmfAjGiVpTN56TXi6SBQtstit5BEw3sijKj1Qkxn6EXKzJ"), Size: "14"},
				{Name: "t/abc", Path: p("QmU7nuGs2djqK99UNsNgEPGh6GV4662p6WtsgccBNGTDxt"), Size: "62"},
				{Name: "t", Path: p("QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr"), Size: "229"},
			},
			wrap: "t",
		},
		{
			name: "progress1M",
			data: func() files.Node {
				return files.NewReaderFile(bytes.NewReader(bytes.Repeat([]byte{0}, 1000000)))
			},
			path: "/ipfs/QmXXNNbwe4zzpdMg62ZXvnX1oU7MwSrQ3vAEtuwFKCm1oD",
			events: []coreiface.AddEvent{
				{Name: "", Bytes: 262144},
				{Name: "", Bytes: 524288},
				{Name: "", Bytes: 786432},
				{Name: "", Bytes: 1000000},
				{Name: "QmXXNNbwe4zzpdMg62ZXvnX1oU7MwSrQ3vAEtuwFKCm1oD", Path: p("QmXXNNbwe4zzpdMg62ZXvnX1oU7MwSrQ3vAEtuwFKCm1oD"), Size: "1000256"},
			},
			wrap: "",
			opts: []options.UnixfsAddOption{options.Unixfs.Progress(true)},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			// recursive logic

			data := testCase.data()
			if testCase.wrap != "" {
				data = files.NewMapDirectory(map[string]files.Node{
					testCase.wrap: data,
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

						if expected[0].Path != nil && event.Path != nil {
							if expected[0].Path.Cid().String() != event.Path.Cid().String() {
								t.Errorf("Event.Hash didn't match, %s != %s", expected[0].Path, event.Path)
							}
						} else if event.Path != expected[0].Path {
							t.Errorf("Event.Hash didn't match, %s != %s", expected[0].Path, event.Path)
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

			tapi, err := api.WithOptions(testCase.apiOpts...)
			if err != nil {
				t.Fatal(err)
			}

			// Add!

			p, err := tapi.Unixfs().Add(ctx, data, opts...)
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

			var cmpFile func(origName string, orig files.Node, gotName string, got files.Node)
			cmpFile = func(origName string, orig files.Node, gotName string, got files.Node) {
				_, origDir := orig.(files.Directory)
				_, gotDir := got.(files.Directory)

				if origDir != gotDir {
					t.Fatal("file type mismatch")
				}

				if origName != gotName {
					t.Errorf("file name mismatch, orig='%s', got='%s'", origName, gotName)
				}

				if !gotDir {
					defer orig.Close()
					defer got.Close()

					do, err := ioutil.ReadAll(orig.(files.File))
					if err != nil {
						t.Fatal(err)
					}

					dg, err := ioutil.ReadAll(got.(files.File))
					if err != nil {
						t.Fatal(err)
					}

					if !bytes.Equal(do, dg) {
						t.Fatal("data not equal")
					}

					return
				}

				origIt := orig.(files.Directory).Entries()
				gotIt := got.(files.Directory).Entries()

				for {
					if origIt.Next() {
						if !gotIt.Next() {
							t.Fatal("gotIt out of entries before origIt")
						}
					} else {
						if gotIt.Next() {
							t.Fatal("origIt out of entries before gotIt")
						}
						break
					}

					cmpFile(origIt.Name(), origIt.Node(), gotIt.Name(), gotIt.Node())
				}
				if origIt.Err() != nil {
					t.Fatal(origIt.Err())
				}
				if gotIt.Err() != nil {
					t.Fatal(gotIt.Err())
				}
			}

			f, err := tapi.Unixfs().Get(ctx, p)
			if err != nil {
				t.Fatal(err)
			}

			orig := testCase.data()
			if testCase.expect != nil {
				orig = testCase.expect(orig)
			}

			cmpFile("", orig, "", f)
		})
	}
}

func (tp *provider) TestAddPinned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
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

func (tp *provider) TestAddHashOnly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
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
	if !strings.Contains(err.Error(), "blockservice: key not found") {
		t.Errorf("unxepected error: %s", err.Error())
	}
}

func (tp *provider) TestGetEmptyFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.Unixfs().Add(ctx, files.NewBytesFile([]byte{}))
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
	n, err := io.ReadFull(r.(files.File), buf)
	if err != nil && err != io.EOF {
		t.Error(err)
	}
	if !bytes.HasPrefix(buf, []byte{0x00}) {
		t.Fatalf("expected empty data, got [%s] [read=%d]", buf, n)
	}
}

func (tp *provider) TestGetDir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}
	edir := unixfs.EmptyDirNode()
	err = api.Dag().Add(ctx, edir)
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

	if _, ok := r.(files.Directory); !ok {
		t.Fatalf("expected a directory")
	}
}

func (tp *provider) TestGetNonUnixfs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	nd := new(mdag.ProtoNode)
	err = api.Dag().Add(ctx, nd)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Unixfs().Get(ctx, coreiface.IpfsPath(nd.Cid()))
	if !strings.Contains(err.Error(), "proto: required field") {
		t.Fatalf("expected protobuf error, got: %s", err)
	}
}

func (tp *provider) TestLs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	r := strings.NewReader("content-of-file")
	p, err := api.Unixfs().Add(ctx, files.NewMapDirectory(map[string]files.Node{
		"0": files.NewMapDirectory(map[string]files.Node{
			"name-of-file":    files.NewReaderFile(r),
			"name-of-symlink": files.NewLinkFile("/foo/bar", nil),
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}

	entries, err := api.Unixfs().Ls(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	entry := <-entries
	if entry.Err != nil {
		t.Fatal(entry.Err)
	}
	if entry.Size != 15 {
		t.Errorf("expected size = 15, got %d", entry.Size)
	}
	if entry.Name != "name-of-file" {
		t.Errorf("expected name = name-of-file, got %s", entry.Name)
	}
	if entry.Type != coreiface.TFile {
		t.Errorf("wrong type %s", entry.Type)
	}
	if entry.Cid.String() != "QmX3qQVKxDGz3URVC3861Z3CKtQKGBn6ffXRBBWGMFz9Lr" {
		t.Errorf("expected cid = QmX3qQVKxDGz3URVC3861Z3CKtQKGBn6ffXRBBWGMFz9Lr, got %s", entry.Cid)
	}
	entry = <-entries
	if entry.Err != nil {
		t.Fatal(entry.Err)
	}
	if entry.Type != coreiface.TSymlink {
		t.Errorf("wrong type %s", entry.Type)
	}
	if entry.Name != "name-of-symlink" {
		t.Errorf("expected name = name-of-symlink, got %s", entry.Name)
	}
	if entry.Target != "/foo/bar" {
		t.Errorf("expected symlink target to be /foo/bar, got %s", entry.Target)
	}

	if l, ok := <-entries; ok {
		t.Errorf("didn't expect a second link")
		if l.Err != nil {
			t.Error(l.Err)
		}
	}
}

func (tp *provider) TestEntriesExpired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	r := strings.NewReader("content-of-file")
	p, err := api.Unixfs().Add(ctx, files.NewMapDirectory(map[string]files.Node{
		"0": files.NewMapDirectory(map[string]files.Node{
			"name-of-file": files.NewReaderFile(r),
		}),
	}))
	if err != nil {
		t.Error(err)
	}

	ctx, cancel = context.WithCancel(ctx)

	nd, err := api.Unixfs().Get(ctx, p)
	if err != nil {
		t.Error(err)
	}
	cancel()

	it := files.ToDir(nd).Entries()
	if it == nil {
		t.Fatal("it was nil")
	}

	if it.Next() {
		t.Fatal("Next succeeded")
	}

	if it.Err() != context.Canceled {
		t.Fatalf("unexpected error %s", it.Err())
	}

	if it.Next() {
		t.Fatal("Next succeeded")
	}
}

func (tp *provider) TestLsEmptyDir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Unixfs().Add(ctx, files.NewMapDirectory(map[string]files.Node{"0": files.NewSliceDirectory([]files.DirEntry{})}))
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
func (tp *provider) TestLsNonUnixfs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	nd, err := cbor.WrapObject(map[string]interface{}{"foo": "bar"}, math.MaxUint64, -1)
	if err != nil {
		t.Fatal(err)
	}

	err = api.Dag().Add(ctx, nd)
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

type closeTestF struct {
	files.File
	closed bool

	t *testing.T
}

type closeTestD struct {
	files.Directory
	closed bool

	t *testing.T
}

func (f *closeTestD) Close() error {
	if f.closed {
		f.t.Fatal("already closed")
	}
	f.closed = true
	return nil
}

func (f *closeTestF) Close() error {
	if f.closed {
		f.t.Fatal("already closed")
	}
	f.closed = true
	return nil
}

func (tp *provider) TestAddCloses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	n4 := &closeTestF{files.NewBytesFile([]byte("foo")), false, t}
	d3 := &closeTestD{files.NewMapDirectory(map[string]files.Node{
		"sub": n4,
	}), false, t}
	n2 := &closeTestF{files.NewBytesFile([]byte("bar")), false, t}
	n1 := &closeTestF{files.NewBytesFile([]byte("baz")), false, t}
	d0 := &closeTestD{files.NewMapDirectory(map[string]files.Node{
		"a": d3,
		"b": n1,
		"c": n2,
	}), false, t}

	_, err = api.Unixfs().Add(ctx, d0)
	if err != nil {
		t.Error(err)
	}

	d0.Close() // Adder doesn't close top-level file

	for i, n := range []*closeTestF{n1, n2, n4} {
		if !n.closed {
			t.Errorf("file %d not closed!", i)
		}
	}

	for i, n := range []*closeTestD{d0, d3} {
		if !n.closed {
			t.Errorf("dir %d not closed!", i)
		}
	}
}

func (tp *provider) TestGetSeek(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	dataSize := int64(100000)
	tf := files.NewReaderFile(io.LimitReader(rand.New(rand.NewSource(1403768328)), dataSize))

	p, err := api.Unixfs().Add(ctx, tf, options.Unixfs.Chunker("size-100"))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Unixfs().Get(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	f := files.ToFile(r)
	if f == nil {
		t.Fatal("not a file")
	}

	orig := make([]byte, dataSize)
	if _, err := io.ReadFull(f, orig); err != nil {
		t.Fatal(err)
	}
	f.Close()

	origR := bytes.NewReader(orig)

	r, err = api.Unixfs().Get(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	f = files.ToFile(r)
	if f == nil {
		t.Fatal("not a file")
	}

	test := func(offset int64, whence int, read int, expect int64, shouldEof bool) {
		t.Run(fmt.Sprintf("seek%d+%d-r%d-%d", whence, offset, read, expect), func(t *testing.T) {
			n, err := f.Seek(offset, whence)
			if err != nil {
				t.Fatal(err)
			}
			origN, err := origR.Seek(offset, whence)
			if err != nil {
				t.Fatal(err)
			}

			if n != origN {
				t.Fatalf("offsets didn't match, expected %d, got %d", origN, n)
			}

			buf := make([]byte, read)
			origBuf := make([]byte, read)
			origRead, err := origR.Read(origBuf)
			if err != nil {
				t.Fatalf("orig: %s", err)
			}
			r, err := io.ReadFull(f, buf)
			switch {
			case shouldEof && err != nil && err != io.ErrUnexpectedEOF:
				fallthrough
			case !shouldEof && err != nil:
				t.Fatalf("f: %s", err)
			case shouldEof:
				_, err := f.Read([]byte{0})
				if err != io.EOF {
					t.Fatal("expected EOF")
				}
				_, err = origR.Read([]byte{0})
				if err != io.EOF {
					t.Fatal("expected EOF (orig)")
				}
			}

			if int64(r) != expect {
				t.Fatal("read wrong amount of data")
			}
			if r != origRead {
				t.Fatal("read different amount of data than bytes.Reader")
			}
			if !bytes.Equal(buf, origBuf) {
				fmt.Fprintf(os.Stderr, "original:\n%s\n", hex.Dump(origBuf))
				fmt.Fprintf(os.Stderr, "got:\n%s\n", hex.Dump(buf))
				t.Fatal("data didn't match")
			}
		})
	}

	test(3, io.SeekCurrent, 10, 10, false)
	test(3, io.SeekCurrent, 10, 10, false)
	test(500, io.SeekCurrent, 10, 10, false)
	test(350, io.SeekStart, 100, 100, false)
	test(-123, io.SeekCurrent, 100, 100, false)
	test(0, io.SeekStart, int(dataSize), dataSize, false)
	test(dataSize-50, io.SeekStart, 100, 50, true)
	test(-5, io.SeekEnd, 100, 5, true)
}
