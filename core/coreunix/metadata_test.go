package coreunix

import (
	"bytes"
	"io/ioutil"
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	core "github.com/jbenet/go-ipfs/core"
	offline "github.com/jbenet/go-ipfs/exchange/offline"
	importer "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
)

func getDagserv(t *testing.T) merkledag.DAGService {
	db := dssync.MutexWrap(ds.NewMapDatastore())
	bs := bstore.NewBlockstore(db)
	blockserv, err := bserv.New(bs, offline.Exchange(bs))
	if err != nil {
		t.Fatal(err)
	}
	return merkledag.NewDAGService(blockserv)
}

func TestMetadata(t *testing.T) {
	// Make some random node
	ds := getDagserv(t)
	data := make([]byte, 1000)
	u.NewTimeSeededRand().Read(data)
	r := bytes.NewReader(data)
	nd, err := importer.BuildDagFromReader(r, ds, nil, chunk.DefaultSplitter)
	if err != nil {
		t.Fatal(err)
	}

	k, err := nd.Key()
	if err != nil {
		t.Fatal(err)
	}

	m := new(ft.Metadata)
	m.MimeType = "THIS IS A TEST"

	// Such effort, many compromise
	ipfsnode := &core.IpfsNode{DAG: ds}

	mdk, err := AddMetadataTo(ipfsnode, k.B58String(), m)
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

	retnode, err := ds.Get(u.B58KeyDecode(mdk))
	if err != nil {
		t.Fatal(err)
	}

	ndr, err := uio.NewDagReader(context.TODO(), retnode, ds)
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
