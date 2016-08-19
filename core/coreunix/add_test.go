package coreunix

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/commands/files"
	"github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin/gc"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/thirdparty/testutil"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func TestAddRecursive(t *testing.T) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: "Qmfoo", // required by offline node
			},
		},
		D: testutil.ThreadSafeCloserMapDatastore(),
	}
	node, err := core.NewNode(context.Background(), &core.BuildCfg{Repo: r})
	if err != nil {
		t.Fatal(err)
	}
	if k, err := AddR(node, "test_data"); err != nil {
		t.Fatal(err)
	} else if k != "QmWCCga8AbTyfAQ7pTnGT6JgmRMAB3Qp8ZmTEFi5q5o8jC" {
		t.Fatal("keys do not match: ", k)
	}
}

func TestAddGCLive(t *testing.T) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: "Qmfoo", // required by offline node
			},
		},
		D: testutil.ThreadSafeCloserMapDatastore(),
	}
	node, err := core.NewNode(context.Background(), &core.BuildCfg{Repo: r})
	if err != nil {
		t.Fatal(err)
	}

	errs := make(chan error)
	out := make(chan interface{})
	adder, err := NewAdder(context.Background(), node.Pinning, node.Blockstore, node.DAG, true)
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

	var gcout <-chan key.Key
	gcstarted := make(chan struct{})
	go func() {
		defer close(gcstarted)
		gcchan, err := gc.GC(context.Background(), node.Blockstore, node.LinkService, node.Pinning, nil)
		if err != nil {
			log.Error("GC ERROR:", err)
			errs <- err
			return
		}

		gcout = gcchan
	}()

	// gc shouldnt start until we let the add finish its current file.
	pipew.Write([]byte("some data for file b"))

	select {
	case <-gcstarted:
		t.Fatal("gc shouldnt have started yet")
	case err := <-errs:
		t.Fatal(err)
	default:
	}

	time.Sleep(time.Millisecond * 100) // make sure gc gets to requesting lock

	// finish write and unblock gc
	pipew.Close()

	// receive next object from adder
	select {
	case o := <-out:
		addedHashes[o.(*AddedObject).Hash] = struct{}{}
	case err := <-errs:
		t.Fatal(err)
	}

	select {
	case <-gcstarted:
	case err := <-errs:
		t.Fatal(err)
	}

	for k := range gcout {
		if _, ok := addedHashes[k.B58String()]; ok {
			t.Fatal("gc'ed a hash we just added")
		}
	}

	var last key.Key
	for a := range out {
		// wait for it to finish
		last = key.B58KeyDecode(a.(*AddedObject).Hash)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	root, err := node.DAG.Get(ctx, last)
	if err != nil {
		t.Fatal(err)
	}

	err = dag.EnumerateChildren(ctx, node.DAG, root.Links, key.NewKeySet(), false)
	if err != nil {
		t.Fatal(err)
	}
}
