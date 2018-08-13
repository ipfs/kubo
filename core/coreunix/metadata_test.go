package coreunix

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	bserv "gx/ipfs/QmSQDddUJRZCBEcEKp3icdiRUrVofvMSWmGyMXSysqjNCL/go-blockservice"
	ft "gx/ipfs/QmTbas51oodp3ZJrqsWYs1yqSxcD7LEJBv4djRV2VrY8wv/go-unixfs"
	importer "gx/ipfs/QmTbas51oodp3ZJrqsWYs1yqSxcD7LEJBv4djRV2VrY8wv/go-unixfs/importer"
	uio "gx/ipfs/QmTbas51oodp3ZJrqsWYs1yqSxcD7LEJBv4djRV2VrY8wv/go-unixfs/io"
	merkledag "gx/ipfs/QmXhrNaxjxNLwAHnWQScc6GxvpJMyn8wfdRmGDbUQwpfth/go-merkledag"

	offline "gx/ipfs/QmNxamk8jB6saNF5G37Tiipf2JEnxt7qvesbngPMe24wMS/go-ipfs-exchange-offline"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	ipld "gx/ipfs/QmUSyMZ8Vt4vTZr5HdDEgEfpwAXfQRuDdfCFTt7XBzhxpQ/go-ipld-format"
	bstore "gx/ipfs/QmYir8arYJK1RMzDBZU1dqscLorWvthWU8aStL4xhxdYeT/go-ipfs-blockstore"
	cid "gx/ipfs/Qmdu2AYUV7yMoVBQPxXNfe7FJcdx16kYtsx6jAPKWQYF1y/go-cid"
	chunker "gx/ipfs/Qme4ThG6LN6EMrMYyf2AMywAZaGbTYxQu4njfcSSkcisLi/go-ipfs-chunker"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
	dssync "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore/sync"
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
