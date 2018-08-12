package coreunix

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/pin/gc"
	"github.com/ipfs/go-ipfs/repo"

	files "gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit/files"
	config "gx/ipfs/QmQSG7YCizeUH2bWatzp6uK9Vm3m7LA5jpxGa9QqgpNKw4/go-ipfs-config"
	dag "gx/ipfs/QmQzSpSjkdGHW6WFBhUG6P3t9K8yv7iucucT1cQaqJ6tgd/go-merkledag"
	blocks "gx/ipfs/QmR54CzE4UcdFAZDehj6HFyy3eSHhVsJUpjfnhCmscuStS/go-block-format"
	"gx/ipfs/QmTZZrpd9o4vpYr9TEADW2EoJ9fzUtAgpXqjxZHbKR2T15/go-blockservice"
	datastore "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	syncds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore/sync"
	blockstore "gx/ipfs/QmYBEfMSquSGnuxBthUoBJNs3F6p4VAPPvAgxq6XXGvTPh/go-ipfs-blockstore"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	pi "gx/ipfs/QmdBpJ5VTfL79VwKDU93z7fyZJ3mm4UaBHrE73CWRw2Bjd/go-ipfs-posinfo"
)

const testPeerID = "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe"

func TestAddRecursive(t *testing.T) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: testPeerID, // required by offline node
			},
		},
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
	}
	node, err := core.NewNode(context.Background(), &core.BuildCfg{Repo: r})
	if err != nil {
		t.Fatal(err)
	}
	if k, err := AddR(node, "test/data"); err != nil {
		t.Fatal(err)
	} else if k != "QmWCCga8AbTyfAQ7pTnGT6JgmRMAB3Qp8ZmTEFi5q5o8jC" {
		t.Fatal("keys do not match: ", k)
	}
}

func TestAddGCLive(t *testing.T) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: testPeerID, // required by offline node
			},
		},
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
	}
	node, err := core.NewNode(context.Background(), &core.BuildCfg{Repo: r})
	if err != nil {
		t.Fatal(err)
	}

	out := make(chan interface{})
	adder, err := NewAdder(context.Background(), node.Pinning, node.Blockstore, node.DAG)
	if err != nil {
		t.Fatal(err)
	}
	adder.Out = out

	dataa := ioutil.NopCloser(bytes.NewBufferString("testfileA"))
	rfa := files.NewReaderFile("a", "a", dataa, nil)

	// make two files with pipes so we can 'pause' the add for timing of the test
	piper, pipew := io.Pipe()
	hangfile := files.NewReaderFile("b", "b", piper, nil)

	datad := ioutil.NopCloser(bytes.NewBufferString("testfileD"))
	rfd := files.NewReaderFile("d", "d", datad, nil)

	slf := files.NewSliceFile("files", "files", []files.File{rfa, hangfile, rfd})

	addDone := make(chan struct{})
	go func() {
		defer close(addDone)
		defer close(out)
		err := adder.AddFile(slf)

		if err != nil {
			t.Fatal(err)
		}

	}()

	addedHashes := make(map[string]struct{})
	select {
	case o := <-out:
		addedHashes[o.(*AddedObject).Hash] = struct{}{}
	case <-addDone:
		t.Fatal("add shouldnt complete yet")
	}

	var gcout <-chan gc.Result
	gcstarted := make(chan struct{})
	go func() {
		defer close(gcstarted)
		gcout = gc.GC(context.Background(), node.Blockstore, node.Repo.Datastore(), node.Pinning, nil)
	}()

	// gc shouldnt start until we let the add finish its current file.
	pipew.Write([]byte("some data for file b"))

	select {
	case <-gcstarted:
		t.Fatal("gc shouldnt have started yet")
	default:
	}

	time.Sleep(time.Millisecond * 100) // make sure gc gets to requesting lock

	// finish write and unblock gc
	pipew.Close()

	// receive next object from adder
	o := <-out
	addedHashes[o.(*AddedObject).Hash] = struct{}{}

	<-gcstarted

	for r := range gcout {
		if r.Error != nil {
			t.Fatal(err)
		}
		if _, ok := addedHashes[r.KeyRemoved.String()]; ok {
			t.Fatal("gc'ed a hash we just added")
		}
	}

	var last *cid.Cid
	for a := range out {
		// wait for it to finish
		c, err := cid.Decode(a.(*AddedObject).Hash)
		if err != nil {
			t.Fatal(err)
		}
		last = c
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	set := cid.NewSet()
	err = dag.EnumerateChildren(ctx, dag.GetLinksWithDAG(node.DAG), last, set.Visit)
	if err != nil {
		t.Fatal(err)
	}
}

func testAddWPosInfo(t *testing.T, rawLeaves bool) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: testPeerID, // required by offline node
			},
		},
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
	}
	node, err := core.NewNode(context.Background(), &core.BuildCfg{Repo: r})
	if err != nil {
		t.Fatal(err)
	}

	bs := &testBlockstore{GCBlockstore: node.Blockstore, expectedPath: "/tmp/foo.txt", t: t}
	bserv := blockservice.New(bs, node.Exchange)
	dserv := dag.NewDAGService(bserv)
	adder, err := NewAdder(context.Background(), node.Pinning, bs, dserv)
	if err != nil {
		t.Fatal(err)
	}
	adder.Out = make(chan interface{})
	adder.Progress = true
	adder.RawLeaves = rawLeaves
	adder.NoCopy = true

	data := make([]byte, 5*1024*1024)
	rand.New(rand.NewSource(2)).Read(data) // Rand.Read never returns an error
	fileData := ioutil.NopCloser(bytes.NewBuffer(data))
	fileInfo := dummyFileInfo{"foo.txt", int64(len(data)), time.Now()}
	file := files.NewReaderFile("foo.txt", "/tmp/foo.txt", fileData, &fileInfo)

	go func() {
		defer close(adder.Out)
		err = adder.AddFile(file)
		if err != nil {
			t.Fatal(err)
		}
	}()
	for range adder.Out {
	}

	exp := 0
	nonOffZero := 0
	if rawLeaves {
		exp = 1
		nonOffZero = 19
	}
	if bs.countAtOffsetZero != exp {
		t.Fatalf("expected %d blocks with an offset at zero (one root and one leafh), got %d", exp, bs.countAtOffsetZero)
	}
	if bs.countAtOffsetNonZero != nonOffZero {
		// note: the exact number will depend on the size and the sharding algo. used
		t.Fatalf("expected %d blocks with an offset > 0, got %d", nonOffZero, bs.countAtOffsetNonZero)
	}
}

func TestAddWPosInfo(t *testing.T) {
	testAddWPosInfo(t, false)
}

func TestAddWPosInfoAndRawLeafs(t *testing.T) {
	testAddWPosInfo(t, true)
}

type testBlockstore struct {
	blockstore.GCBlockstore
	expectedPath         string
	t                    *testing.T
	countAtOffsetZero    int
	countAtOffsetNonZero int
}

func (bs *testBlockstore) Put(block blocks.Block) error {
	bs.CheckForPosInfo(block)
	return bs.GCBlockstore.Put(block)
}

func (bs *testBlockstore) PutMany(blocks []blocks.Block) error {
	for _, blk := range blocks {
		bs.CheckForPosInfo(blk)
	}
	return bs.GCBlockstore.PutMany(blocks)
}

func (bs *testBlockstore) CheckForPosInfo(block blocks.Block) error {
	fsn, ok := block.(*pi.FilestoreNode)
	if ok {
		posInfo := fsn.PosInfo
		if posInfo.FullPath != bs.expectedPath {
			bs.t.Fatal("PosInfo does not have the expected path")
		}
		if posInfo.Offset == 0 {
			bs.countAtOffsetZero += 1
		} else {
			bs.countAtOffsetNonZero += 1
		}
	}
	return nil
}

type dummyFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (fi *dummyFileInfo) Name() string       { return fi.name }
func (fi *dummyFileInfo) Size() int64        { return fi.size }
func (fi *dummyFileInfo) Mode() os.FileMode  { return 0 }
func (fi *dummyFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *dummyFileInfo) IsDir() bool        { return false }
func (fi *dummyFileInfo) Sys() interface{}   { return nil }
