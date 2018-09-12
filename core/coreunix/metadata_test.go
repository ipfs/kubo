package coreunix

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	ft "gx/ipfs/QmPXzQ9LAFGZjcifFANCQFQiYt5SXgJziGoxUfJULVpHyA/go-unixfs"
	importer "gx/ipfs/QmPXzQ9LAFGZjcifFANCQFQiYt5SXgJziGoxUfJULVpHyA/go-unixfs/importer"
	uio "gx/ipfs/QmPXzQ9LAFGZjcifFANCQFQiYt5SXgJziGoxUfJULVpHyA/go-unixfs/io"
	merkledag "gx/ipfs/QmURqt1jB9Yu3X4Tr9WQJf36QGN7vi8mGTzjnX2ij1CJwC/go-merkledag"
	bserv "gx/ipfs/QmYHXfGs5GVxXN233aFr5Jenvd7NG4qZ7pmjfyz7yvG93m/go-blockservice"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	ds "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore"
	dssync "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore/sync"
	offline "gx/ipfs/QmXHsHBveZF6ueKzDJbUg476gmrbzoR1yijiyH5SZAEuDT/go-ipfs-exchange-offline"
	ipld "gx/ipfs/QmdDXJs4axxefSPgK6Y1QhpJWKuDPnGJiqgq4uncb4rFHL/go-ipld-format"
	chunker "gx/ipfs/QmdSeG9s4EQ9TGruJJS9Us38TQDZtMmFGwzTYUDVqNTURm/go-ipfs-chunker"
	bstore "gx/ipfs/QmeMussyD8s3fQ3pM19ZsfbxvomEqPV9FvczLMWyBDYSnS/go-ipfs-blockstore"
)

func getDagserv(t *testing.T) ipld.DAGService {
	db := dssync.MutexWrap(ds.NewMapDatastore())
	bs := bstore.NewBlockstore(db)
	blockserv := bserv.New(bs, offline.Exchange(bs))
	return merkledag.NewDAGService(blockserv)
}

func TestMetadata(t *testing.T) {
	ctx := context.Background()
	// Make some random node
	ds := getDagserv(t)
	data := make([]byte, 1000)
	u.NewTimeSeededRand().Read(data)
	r := bytes.NewReader(data)
	nd, err := importer.BuildDagFromReader(ds, chunker.DefaultSplitter(r))
	if err != nil {
		t.Fatal(err)
	}

	c := nd.Cid()

	m := new(ft.Metadata)
	m.MimeType = "THIS IS A TEST"

	// Such effort, many compromise
	ipfsnode := &core.IpfsNode{DAG: ds}

	mdk, err := AddMetadataTo(ipfsnode, c.String(), m)
	if err != nil {
		t.Fatal(err)
	}

	rec, err := Metadata(ipfsnode, mdk)
	if err != nil {
		t.Fatal(err)
	}
	if rec.MimeType != m.MimeType {
		t.Fatalf("something went wrong in conversion: '%s' != '%s'", rec.MimeType, m.MimeType)
	}

	cdk, err := cid.Decode(mdk)
	if err != nil {
		t.Fatal(err)
	}

	retnode, err := ds.Get(ctx, cdk)
	if err != nil {
		t.Fatal(err)
	}

	rtnpb, ok := retnode.(*merkledag.ProtoNode)
	if !ok {
		t.Fatal("expected protobuf node")
	}

	ndr, err := uio.NewDagReader(ctx, rtnpb, ds)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(ndr)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, data) {
		t.Fatal("read incorrect data")
	}
}
