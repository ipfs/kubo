package io

import (
	"io/ioutil"
	"testing"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	testu "github.com/ipfs/go-ipfs/unixfs/test"
)

func TestBasicRead(t *testing.T) {
	dserv := testu.GetDAGServ()
	inbuf, node := testu.GetRandomNode(t, dserv, 1024)
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

	err = testu.ArrComp(inbuf, outbuf)
	if err != nil {
		t.Fatal(err)
	}
}
