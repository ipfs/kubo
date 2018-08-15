package coreunix

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	merkledag "gx/ipfs/QmQzSpSjkdGHW6WFBhUG6P3t9K8yv7iucucT1cQaqJ6tgd/go-merkledag"
	bserv "gx/ipfs/QmTZZrpd9o4vpYr9TEADW2EoJ9fzUtAgpXqjxZHbKR2T15/go-blockservice"
	ft "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs"
	importer "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs/importer"
	uio "gx/ipfs/QmWv8MYwgPK4zXYv1et1snWJ6FWGqaL6xY2y9X1bRSKBxk/go-unixfs/io"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	ds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	dssync "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore/sync"
	offline "gx/ipfs/QmVozMmsgK2PYyaHQsrcWLBYigb1m6mW8YhCBG2Cb4Uxq9/go-ipfs-exchange-offline"
	chunker "gx/ipfs/QmWbCAB5f3LDumj4ncz1UCHSiyXrXxkMxZB6Wv35xi4P8z/go-ipfs-chunker"
	bstore "gx/ipfs/QmYBEfMSquSGnuxBthUoBJNs3F6p4VAPPvAgxq6XXGvTPh/go-ipfs-blockstore"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
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
