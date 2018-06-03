package coreunix

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	core "github.com/ipfs/go-ipfs/core"
	importer "github.com/ipfs/go-ipfs/importer"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	offline "gx/ipfs/QmYk9mQ4iByLLFzZPGWMnjJof3DQ3QneFFR6ZtNAXd8UvS/go-ipfs-exchange-offline"
	bstore "gx/ipfs/QmayRSLCiM2gWR7Kay8vqu3Yy5mf7yPqocF9ZRgDUPYMcc/go-ipfs-blockstore"
	chunker "gx/ipfs/QmbGDSVKnYJZrtUnyxwsUpCeuigshNuVFxXCpv13jXecq1/go-ipfs-chunker"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
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
