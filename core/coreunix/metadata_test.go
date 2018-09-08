package coreunix

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	core "github.com/ipfs/go-ipfs/core"
	merkledag "gx/ipfs/QmNr4E8z9bGTztvHJktp7uQaMdx9p3r9Asrq6eYk7iCh4a/go-merkledag"
	bserv "gx/ipfs/QmQLG22wSEStiociTSKQpZAuuaaWoF1B3iKyjPFvWiTQ77/go-blockservice"
	ft "gx/ipfs/QmWAfTyD6KEBm7bzqNRBPvqKrZCDtn5PGbs9V1DKfnVK59/go-unixfs"
	importer "gx/ipfs/QmWAfTyD6KEBm7bzqNRBPvqKrZCDtn5PGbs9V1DKfnVK59/go-unixfs/importer"
	uio "gx/ipfs/QmWAfTyD6KEBm7bzqNRBPvqKrZCDtn5PGbs9V1DKfnVK59/go-unixfs/io"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	offline "gx/ipfs/QmPuLWvxK1vg6ckKUpT53Dow9VLCcQGdL5Trwxa8PTLp7r/go-ipfs-exchange-offline"
	ds "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore"
	dssync "gx/ipfs/QmSpg1CvpXQQow5ernt1gNBXaXV6yxyNqi7XoeerWfzB5w/go-datastore/sync"
	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	chunker "gx/ipfs/QmXzBbJo2sLf3uwjNTeoWYiJV7CjAhkiA4twtLvwJSSNdK/go-ipfs-chunker"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	bstore "gx/ipfs/Qmeg56ecxRnVv7VWViMrDeEMoBHaNFMs4vQnyQrJ79Zz7i/go-ipfs-blockstore"
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
