package io

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/ipfs/go-ipfs/blocks/blockstore"
	bs "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	imp "github.com/ipfs/go-ipfs/importer"
	"github.com/ipfs/go-ipfs/importer/chunk"
	mdag "github.com/ipfs/go-ipfs/merkledag"

	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	"gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/sync"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func getMockDagServ(t testing.TB) mdag.DAGService {
	dstore := ds.NewMapDatastore()
	tsds := sync.MutexWrap(dstore)
	bstore := blockstore.NewBlockstore(tsds)
	bserv := bs.New(bstore, offline.Exchange(bstore))
	return mdag.NewDAGService(bserv)
}

func sizeSplitterGen(size int64) chunk.SplitterGen {
	return func(r io.Reader) chunk.Splitter {
		return chunk.NewSizeSplitter(r, size)
	}
}

func getNode(t testing.TB, dserv mdag.DAGService, data []byte) *mdag.Node {
	in := bytes.NewReader(data)
	node, err := imp.BuildTrickleDagFromReader(dserv, sizeSplitterGen(500)(in))
	if err != nil {
		t.Fatal(err)
	}

	return node
}

func getRandomNode(t testing.TB, dserv mdag.DAGService, size int64) ([]byte, *mdag.Node) {
	in := io.LimitReader(u.NewTimeSeededRand(), size)
	buf, err := ioutil.ReadAll(in)
	if err != nil {
		t.Fatal(err)
	}

	node := getNode(t, dserv, buf)
	return buf, node
}

func arrComp(a, b []byte) error {
	if len(a) != len(b) {
		return fmt.Errorf("Arrays differ in length. %d != %d", len(a), len(b))
	}
	for i, v := range a {
		if v != b[i] {
			return fmt.Errorf("Arrays differ at index: %d", i)
		}
	}
	return nil
}

func TestBasicRead(t *testing.T) {
	dserv := getMockDagServ(t)
	inbuf, node := getRandomNode(t, dserv, 1024)
	ctx, closer := context.WithCancel(context.Background())
	defer closer()

	reader, err := NewDagReader(ctx, node, dserv)
	if err != nil {
		t.Fatal(err)
	}

	outbuf, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(inbuf, outbuf)
	if err != nil {
		t.Fatal(err)
	}
}
