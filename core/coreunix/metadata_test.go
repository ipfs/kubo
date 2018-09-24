package coreunix

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	ft "gx/ipfs/QmU4x3742bvgfxJsByEDpBnifJqjJdV6x528co4hwKCn46/go-unixfs"
	importer "gx/ipfs/QmU4x3742bvgfxJsByEDpBnifJqjJdV6x528co4hwKCn46/go-unixfs/importer"
	uio "gx/ipfs/QmU4x3742bvgfxJsByEDpBnifJqjJdV6x528co4hwKCn46/go-unixfs/io"
	merkledag "gx/ipfs/QmcBoNcAP6qDjgRBew7yjvCqHq7p5jMstE44jPUBWBxzsV/go-merkledag"
	bserv "gx/ipfs/QmcRecCZWM2NZfCQrCe97Ch3Givv8KKEP82tGUDntzdLFe/go-blockservice"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	offline "gx/ipfs/QmR5miWuikPxWyUrzMYJVmFUcD44pGdtc98h9Qsbp4YcJw/go-ipfs-exchange-offline"
	chunker "gx/ipfs/QmULKgr55cSWR8Kiwy3cVRcAiGVnR6EVSaB7hJcWS4138p/go-ipfs-chunker"
	ds "gx/ipfs/QmUyz7JTJzgegC6tiJrfby3mPhzcdswVtG4x58TQ6pq8jV/go-datastore"
	dssync "gx/ipfs/QmUyz7JTJzgegC6tiJrfby3mPhzcdswVtG4x58TQ6pq8jV/go-datastore/sync"
	ipld "gx/ipfs/QmdDXJs4axxefSPgK6Y1QhpJWKuDPnGJiqgq4uncb4rFHL/go-ipld-format"
	bstore "gx/ipfs/QmdriVJgKx4JADRgh3cYPXqXmsa1A45SvFki1nDWHhQNtC/go-ipfs-blockstore"
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
