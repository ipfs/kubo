package io

import (
	"io/ioutil"
	"os"
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

func TestSeekAndRead(t *testing.T) {
	dserv := testu.GetDAGServ()
	inbuf := make([]byte, 256)
	for i := 0; i <= 255; i++ {
		inbuf[i] = byte(i)
	}

	node := testu.GetNode(t, dserv, inbuf)
	ctx, closer := context.WithCancel(context.Background())
	defer closer()

	reader, err := NewDagReader(ctx, node, dserv)
	if err != nil {
		t.Fatal(err)
	}

	for i := 255; i >= 0; i-- {
		reader.Seek(int64(i), os.SEEK_SET)
		out := make([]byte, 1)

		if reader.Offset() != int64(i) {
			t.Fatal("expected offset to be increased by one after read")
		}

		c, err := reader.Read(out)
		if c != 1 {
			t.Fatal("reader should have read just one byte")
		}
		if err != nil {
			t.Fatal(err)
		}

		if int(out[0]) != i {
			t.Fatalf("read %d at index %d, expected %d", out[0], i, i)
		}

		if reader.Offset() != int64(i+1) {
			t.Fatal("expected offset to be increased by one after read")
		}
	}
}
