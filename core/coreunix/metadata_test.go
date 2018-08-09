package coreunix

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	ft "gx/ipfs/QmWJRM6rLjXGEXb5JkKu17Y68eJtCFcKPyRhb8JH2ELZ2Q/go-unixfs"
	importer "gx/ipfs/QmWJRM6rLjXGEXb5JkKu17Y68eJtCFcKPyRhb8JH2ELZ2Q/go-unixfs/importer"
	uio "gx/ipfs/QmWJRM6rLjXGEXb5JkKu17Y68eJtCFcKPyRhb8JH2ELZ2Q/go-unixfs/io"
	merkledag "gx/ipfs/QmXkZeJmx4c3ddjw81DQMUpM1e5LjAack5idzZYWUb2qAJ/go-merkledag"
	bserv "gx/ipfs/QmeZMtdkNG7u2CohGSL8mzAdZY2c3B1coYE91wvbzip1pF/go-blockservice"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	bstore "gx/ipfs/QmRNFh4wm6FgTDrtsWmnvEP9NTuEa3Ykf72y1LXCyevbGW/go-ipfs-blockstore"
	offline "gx/ipfs/QmWURzU3XRY4wYBsu2LHukKKHp5skkYB1K357nzpbEvRY4/go-ipfs-exchange-offline"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
	chunker "gx/ipfs/Qmc3UwSvJkntxu2gKDPCqJEzmhqVeJtTbrVxJm6tsdmMF1/go-ipfs-chunker"
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
