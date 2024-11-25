package tests

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ipfs/boxo/path"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"

	"github.com/ipfs/boxo/files"
	mdag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
)

func (tp *TestSuite) TestUnixfs(t *testing.T) {
	tp.hasApi(t, func(api coreiface.CoreAPI) error {
		if api.Unixfs() == nil {
			return errAPINotImplemented
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
	t.Run("TestGetReadAt", tp.TestGetReadAt)
}

// `echo -n 'hello, world!' | ipfs add`
var (
	hello    = "/ipfs/QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk"
	helloStr = "hello, world!"
)

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

func wrapped(names ...string) func(f files.Node) files.Node {
	return func(f files.Node) files.Node {
		for i := range names {
			f = files.NewMapDirectory(map[string]files.Node{
				names[len(names)-i-1]: f,
			})
		}
		return f
	}
}

func (tp *TestSuite) TestAdd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	p := func(h string) path.ImmutablePath {
		c, err := cid.Parse(h)
		if err != nil {
			t.Fatal(err)
		}
		return path.FromCid(c)
	}

	rf, err := os.CreateTemp(os.TempDir(), "unixfs-add-real")
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
		n, err := files.NewReaderPathFile(rfp, io.NopCloser(strings.NewReader(helloStr)), stat)
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
			path: "/ipfs/bafkreidi4zlleupgp2bvrpxyja5lbvi4mym7hz5bvhyoowby2qp7g2hxfa",
			opts: []options.UnixfsAddOption{options.Unixfs.CidVersion(1)},
		},
		{
			name: "addCidV1NoLeaves",
			data: strFile(helloStr),
			path: "/ipfs/bafybeibhbcn7k7o2m6xsqkrlfiokod3nxwe47viteynhruh6uqx7hvkjfu",
			opts: []options.UnixfsAddOption{options.Unixfs.CidVersion(1), options.Unixfs.RawLeaves(false)},
		},
		// Non sha256 hash vs CID
		{
			name: "addCidSha3",
			data: strFile(helloStr),
			path: "/ipfs/bafkrmichjflejeh6aren53o7pig7zk3m3vxqcoc2i5dv326k3x6obh7jry",
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
			path: "/ipfs/bafyaafikcmeaeeqnnbswy3dpfqqho33snrsccgan",
			opts: []options.UnixfsAddOption{options.Unixfs.Inline(true)},
		},
		{
			name: "addInlineLimit",
			data: strFile(helloStr),
			path: "/ipfs/bafyaafikcmeaeeqnnbswy3dpfqqho33snrsccgan",
			opts: []options.UnixfsAddOption{options.Unixfs.InlineLimit(32), options.Unixfs.Inline(true)},
		},
		{
			name: "addInlineZero",
			data: strFile(""),
			path: "/ipfs/bafkqaaa",
			opts: []options.UnixfsAddOption{options.Unixfs.InlineLimit(0), options.Unixfs.Inline(true), options.Unixfs.RawLeaves(true)},
		},
		{ // TODO: after coreapi add is used in `ipfs add`, consider making this default for inline
			name: "addInlineRaw",
			data: strFile(helloStr),
			path: "/ipfs/bafkqadlimvwgy3zmeb3w64tmmqqq",
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
			name: "simpleDirNoWrap",
			data: flatDir,
			path: "/ipfs/QmRKGpFfR32FVXdvJiHfo4WJ5TDYBsM1P9raAp1p6APWSp",
		},
		{
			name:   "simpleDir",
			data:   flatDir,
			wrap:   "t",
			expect: wrapped("t"),
			path:   "/ipfs/Qmc3nGXm1HtUVCmnXLQHvWcNwfdZGpfg2SRm1CxLf7Q2Rm",
		},
		{
			name:   "twoLevelDir",
			data:   twoLevelDir(),
			wrap:   "t",
			expect: wrapped("t"),
			path:   "/ipfs/QmPwsL3T5sWhDmmAWZHAzyjKtMVDS9a11aHNRqb3xoVnmg",
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
		},
		// hidden
		{
			name: "hiddenFilesAdded",
			data: func() files.Node {
				return files.NewMapDirectory(map[string]files.Node{
					".bar": files.NewBytesFile([]byte("hello2")),
					"bar":  files.NewBytesFile([]byte("hello2")),
					"foo":  files.NewBytesFile([]byte("hello1")),
				})
			},
			wrap:   "t",
			expect: wrapped("t"),
			path:   "/ipfs/QmPXLSBX382vJDLrGakcbrZDkU3grfkjMox7EgSC9KFbtQ",
		},
		// NoCopy
		{
			name: "simpleNoCopy",
			data: realFile,
			path: "/ipfs/bafkreidi4zlleupgp2bvrpxyja5lbvi4mym7hz5bvhyoowby2qp7g2hxfa",
			opts: []options.UnixfsAddOption{options.Unixfs.Nocopy(true)},
		},
		{
			name: "noCopyNoRaw",
			data: realFile,
			path: "/ipfs/bafkreidi4zlleupgp2bvrpxyja5lbvi4mym7hz5bvhyoowby2qp7g2hxfa",
			opts: []options.UnixfsAddOption{options.Unixfs.Nocopy(true), options.Unixfs.RawLeaves(false)},
			err:  "nocopy option requires '--raw-leaves' to be enabled as well",
		},
		{
			name: "noCopyNoPath",
			data: strFile(helloStr),
			path: "/ipfs/bafkreidi4zlleupgp2bvrpxyja5lbvi4mym7hz5bvhyoowby2qp7g2hxfa",
			opts: []options.UnixfsAddOption{options.Unixfs.Nocopy(true)},
			err:  helpers.ErrMissingFsRef.Error(),
		},
		// Events / Progress
		{
			name: "simpleAddEvent",
			data: strFile(helloStr),
			path: "/ipfs/bafkreidi4zlleupgp2bvrpxyja5lbvi4mym7hz5bvhyoowby2qp7g2hxfa",
			events: []coreiface.AddEvent{
				{Name: "bafkreidi4zlleupgp2bvrpxyja5lbvi4mym7hz5bvhyoowby2qp7g2hxfa", Path: p("bafkreidi4zlleupgp2bvrpxyja5lbvi4mym7hz5bvhyoowby2qp7g2hxfa"), Size: strconv.Itoa(len(helloStr))},
			},
			opts: []options.UnixfsAddOption{options.Unixfs.RawLeaves(true)},
		},
		{
			name: "silentAddEvent",
			data: twoLevelDir(),
			path: "/ipfs/QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr",
			events: []coreiface.AddEvent{
				{Name: "abc", Path: p("QmU7nuGs2djqK99UNsNgEPGh6GV4662p6WtsgccBNGTDxt"), Size: "62"},
				{Name: "", Path: p("QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr"), Size: "229"},
			},
			opts: []options.UnixfsAddOption{options.Unixfs.Silent(true)},
		},
		{
			name: "dirAddEvents",
			data: twoLevelDir(),
			path: "/ipfs/QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr",
			events: []coreiface.AddEvent{
				{Name: "abc/def", Path: p("QmNyJpQkU1cEkBwMDhDNFstr42q55mqG5GE5Mgwug4xyGk"), Size: "13"},
				{Name: "bar", Path: p("QmS21GuXiRMvJKHos4ZkEmQDmRBqRaF5tQS2CQCu2ne9sY"), Size: "14"},
				{Name: "foo", Path: p("QmfAjGiVpTN56TXi6SBQtstit5BEw3sijKj1Qkxn6EXKzJ"), Size: "14"},
				{Name: "abc", Path: p("QmU7nuGs2djqK99UNsNgEPGh6GV4662p6WtsgccBNGTDxt"), Size: "62"},
				{Name: "", Path: p("QmVG2ZYCkV1S4TK8URA3a4RupBF17A8yAr4FqsRDXVJASr"), Size: "229"},
			},
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
							t.Error("unexpected event type")
							continue
						}

						if len(expected) < 1 {
							t.Error("got more events than expected")
							continue
						}

						if expected[0].Size != event.Size {
							t.Errorf("Event.Size didn't match, %s != %s", expected[0].Size, event.Size)
						}

						if expected[0].Name != event.Name {
							t.Errorf("Event.Name didn't match, %s != %s", expected[0].Name, event.Name)
						}

						if (expected[0].Path != path.ImmutablePath{} && event.Path != path.ImmutablePath{}) {
							if expected[0].Path.RootCid().String() != event.Path.RootCid().String() {
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
						t.Errorf("%d event(s) didn't arrive", len(expected))
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

				if origName != gotName {
					t.Errorf("file name mismatch, orig='%s', got='%s'", origName, gotName)
				}

				if origDir != gotDir {
					t.Fatalf("file type mismatch on %s", origName)
				}

				if !gotDir {
					defer orig.Close()
					defer got.Close()

					do, err := io.ReadAll(orig.(files.File))
					if err != nil {
						t.Fatal(err)
					}

					dg, err := io.ReadAll(got.(files.File))
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

func (tp *TestSuite) TestAddPinned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.Unixfs().Add(ctx, strFile(helloStr)(), options.Unixfs.Pin(true))
	if err != nil {
		t.Fatal(err)
	}

	pins, err := accPins(ctx, api)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(pins))
	}

	if pins[0].Path().String() != "/ipfs/QmQy2Dw4Wk7rdJKjThjYXzfFJNaRKRHhHP5gHHXroJMYxk" {
		t.Fatalf("got unexpected pin: %s", pins[0].Path().String())
	}
}

func (tp *TestSuite) TestAddHashOnly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	p, err := api.Unixfs().Add(ctx, strFile(helloStr)(), options.Unixfs.HashOnly(true))
	if err != nil {
		t.Fatal(err)
	}

	if p.String() != hello {
		t.Errorf("unexpected path: %s", p.String())
	}

	_, err = api.Block().Get(ctx, p)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !ipld.IsNotFound(err) {
		t.Errorf("unexpected error: %s", err.Error())
	}
}

func (tp *TestSuite) TestGetEmptyFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.Unixfs().Add(ctx, files.NewBytesFile([]byte{}))
	if err != nil {
		t.Fatal(err)
	}

	emptyFilePath, err := path.NewPath(emptyFile)
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

func (tp *TestSuite) TestGetDir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}
	edir := unixfs.EmptyDirNode()
	err = api.Dag().Add(ctx, edir)
	if err != nil {
		t.Fatal(err)
	}
	p := path.FromCid(edir.Cid())

	if p.String() != path.FromCid(edir.Cid()).String() {
		t.Fatalf("expected path %s, got: %s", edir.Cid(), p.String())
	}

	r, err := api.Unixfs().Get(ctx, path.FromCid(edir.Cid()))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := r.(files.Directory); !ok {
		t.Fatalf("expected a directory")
	}
}

func (tp *TestSuite) TestGetNonUnixfs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	nd := new(mdag.ProtoNode)
	err = api.Dag().Add(ctx, nd)
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.Unixfs().Get(ctx, path.FromCid(nd.Cid()))
	if !strings.Contains(err.Error(), "proto: required field") {
		t.Fatalf("expected protobuf error, got: %s", err)
	}
}

func (tp *TestSuite) TestLs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	r := strings.NewReader("content-of-file")
	p, err := api.Unixfs().Add(ctx, files.NewMapDirectory(map[string]files.Node{
		"name-of-file":    files.NewReaderFile(r),
		"name-of-symlink": files.NewLinkFile("/foo/bar", nil),
	}))
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	entries := make(chan coreiface.DirEntry)
	go func() {
		errCh <- api.Unixfs().Ls(ctx, p, entries)
	}()

	entry, ok := <-entries
	if !ok {
		t.Fatal("expected another entry")
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
	entry, ok = <-entries
	if !ok {
		t.Fatal("expected another entry")
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

	_, ok = <-entries
	if ok {
		t.Errorf("didn't expect a another link")
	}
	if err = <-errCh; err != nil {
		t.Error(err)
	}
}

func (tp *TestSuite) TestEntriesExpired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	r := strings.NewReader("content-of-file")
	p, err := api.Unixfs().Add(ctx, files.NewMapDirectory(map[string]files.Node{
		"name-of-file": files.NewReaderFile(r),
	}))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithCancel(ctx)

	nd, err := api.Unixfs().Get(ctx, p)
	if err != nil {
		t.Fatal(err)
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

func (tp *TestSuite) TestLsEmptyDir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	p, err := api.Unixfs().Add(ctx, files.NewSliceDirectory([]files.DirEntry{}))
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	links := make(chan coreiface.DirEntry)
	go func() {
		errCh <- api.Unixfs().Ls(ctx, p, links)
	}()

	var count int
	for range links {
		count++
	}
	if err = <-errCh; err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("expected 0 links, got %d", count)
	}
}

// TODO(lgierth) this should test properly, with len(links) > 0
func (tp *TestSuite) TestLsNonUnixfs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	nd, err := cbor.WrapObject(map[string]interface{}{"foo": "bar"}, math.MaxUint64, -1)
	if err != nil {
		t.Fatal(err)
	}

	err = api.Dag().Add(ctx, nd)
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	links := make(chan coreiface.DirEntry)
	go func() {
		errCh <- api.Unixfs().Ls(ctx, path.FromCid(nd.Cid()), links)
	}()

	var count int
	for range links {
		count++
	}
	if err = <-errCh; err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("expected 0 links, got %d", count)
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
	f.t.Helper()
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

func (tp *TestSuite) TestAddCloses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}

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

func (tp *TestSuite) TestGetSeek(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
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

func (tp *TestSuite) TestGetReadAt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
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

	f, ok := r.(interface {
		files.File
		io.ReaderAt
	})
	if !ok {
		t.Skip("ReaderAt not implemented")
	}

	orig := make([]byte, dataSize)
	if _, err := io.ReadFull(f, orig); err != nil {
		t.Fatal(err)
	}
	f.Close()

	origR := bytes.NewReader(orig)

	if _, err := api.Unixfs().Get(ctx, p); err != nil {
		t.Fatal(err)
	}

	test := func(offset int64, read int, expect int64, shouldEof bool) {
		t.Run(fmt.Sprintf("readat%d-r%d-%d", offset, read, expect), func(t *testing.T) {
			origBuf := make([]byte, read)
			origRead, err := origR.ReadAt(origBuf, offset)
			if err != nil && err != io.EOF {
				t.Fatalf("orig: %s", err)
			}
			buf := make([]byte, read)
			r, err := f.ReadAt(buf, offset)
			if shouldEof {
				if err != io.EOF {
					t.Fatal("expected EOF, got: ", err)
				}
			} else if err != nil {
				t.Fatal("got: ", err)
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

	test(3, 10, 10, false)
	test(13, 10, 10, false)
	test(513, 10, 10, false)
	test(350, 100, 100, false)
	test(0, int(dataSize), dataSize, false)
	test(dataSize-50, 100, 50, true)
}
