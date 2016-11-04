package io

import (
	"context"
	"io/ioutil"
	"testing"

	testu "github.com/ipfs/go-ipfs/unixfs/test"
)

func TestEmptyNode(t *testing.T) {
	n := NewEmptyDirectory()
	if len(n.Links()) != 0 {
		t.Fatal("empty node should have 0 links")
	}
}

func TestDirBuilder(t *testing.T) {
	dserv := testu.GetDAGServ()
	ctx, closer := context.WithCancel(context.Background())
	defer closer()
	inbuf, node := testu.GetRandomNode(t, dserv, 1024)
	key := node.Cid()

	b := NewDirectory(dserv)

	b.AddChild(ctx, "random", key)

	dir := b.GetNode()
	outn, err := dir.GetLinkedProtoNode(ctx, dserv, "random")
	if err != nil {
		t.Fatal(err)
	}

	reader, err := NewDagReader(ctx, outn, dserv)
	if err != nil {
		t.Fatal(err)
	}

	outbuf, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	err = testu.ArrComp(inbuf, outbuf)
	if err != nil {
		t.Fatal(err)
	}

}
