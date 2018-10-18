package coreunix

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	bserv "gx/ipfs/QmSU7Nx5eUHWkc9zCTiXDu3ZkdXAZdRgRGRaKM86VjGU4m/go-blockservice"
	merkledag "gx/ipfs/QmVvNkTCx8V9Zei8xuTYTBdUXmbnDRS4iNuw1SztYyhQwQ/go-merkledag"
	ft "gx/ipfs/QmWE6Ftsk98cG2MTVgH4wJT8VP2nL9TuBkYTrz9GSqcsh5/go-unixfs"
	importer "gx/ipfs/QmWE6Ftsk98cG2MTVgH4wJT8VP2nL9TuBkYTrz9GSqcsh5/go-unixfs/importer"
	uio "gx/ipfs/QmWE6Ftsk98cG2MTVgH4wJT8VP2nL9TuBkYTrz9GSqcsh5/go-unixfs/io"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	offline "gx/ipfs/QmT6dHGp3UYd3vUMpy7rzX2CXQv7HLcj42Vtq8qwwjgASb/go-ipfs-exchange-offline"
	chunker "gx/ipfs/QmTUTG9Jg9ZRA1EzTPGTDvnwfcfKhDMnqANnP9fe4rSjMR/go-ipfs-chunker"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	dssync "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/sync"
	bstore "gx/ipfs/QmcDDgAXDbpDUpadCJKLr49KYR4HuL7T8Z1dZTHt6ixsoR/go-ipfs-blockstore"
	ipld "gx/ipfs/QmdDXJs4axxefSPgK6Y1QhpJWKuDPnGJiqgq4uncb4rFHL/go-ipld-format"
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
