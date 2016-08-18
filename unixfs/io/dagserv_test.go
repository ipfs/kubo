package io

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	imp "github.com/ipfs/go-ipfs/importer"
	"github.com/ipfs/go-ipfs/importer/chunk"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	mdagmock "github.com/ipfs/go-ipfs/merkledag/test"

	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

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
	dserv := mdagmock.Mock()
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
