package io_test

import (
	"testing"

	"io"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	bs "github.com/jbenet/go-ipfs/blockservice"
	importer "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	dagio "github.com/jbenet/go-ipfs/unixfs/io"
)

type datasource struct {
	i int
}

func (d *datasource) Read(b []byte) (int, error) {
	for i, _ := range b {
		b[i] = byte(d.i % 256)
		d.i++
	}
	return len(b), nil
}

func (d *datasource) Matches(t *testing.T, r io.Reader, length int) bool {
	b := make([]byte, 100)
	i := 0
	for {
		n, err := r.Read(b)
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		for _, v := range b[:n] {
			if v != byte(i%256) {
				t.Fatalf("Buffers differed at byte: %d (%d != %d)", i, v, (i % 256))
			}
			i++
		}
		if err == io.EOF {
			break
		}
	}
	if i != length {
		t.Fatalf("Incorrect length. (%d != %d)", i, length)
	}
	return true
}

func TestDagWriter(t *testing.T) {
	dstore := ds.NewMapDatastore()
	bserv, err := bs.NewBlockService(dstore, nil)
	if err != nil {
		t.Fatal(err)
	}
	dag := mdag.NewDAGService(bserv)
	dw := dagio.NewDagWriter(dag, &chunk.SizeSplitter{Size: 4096})

	nbytes := int64(1024 * 1024 * 2)
	n, err := io.CopyN(dw, &datasource{}, nbytes)
	if err != nil {
		t.Fatal(err)
	}

	if n != nbytes {
		t.Fatal("Copied incorrect amount of bytes!")
	}

	dw.Close()

	node := dw.GetNode()
	read, err := dagio.NewDagReader(node, dag)
	if err != nil {
		t.Fatal(err)
	}

	d := &datasource{}
	if !d.Matches(t, read, int(nbytes)) {
		t.Fatal("Failed to validate!")
	}
}

func TestMassiveWrite(t *testing.T) {
	t.SkipNow()
	dstore := ds.NewNullDatastore()
	bserv, err := bs.NewBlockService(dstore, nil)
	if err != nil {
		t.Fatal(err)
	}
	dag := mdag.NewDAGService(bserv)
	dw := dagio.NewDagWriter(dag, &chunk.SizeSplitter{Size: 4096})

	nbytes := int64(1024 * 1024 * 1024 * 16)
	n, err := io.CopyN(dw, &datasource{}, nbytes)
	if err != nil {
		t.Fatal(err)
	}
	if n != nbytes {
		t.Fatal("Incorrect copy size.")
	}
	dw.Close()
}

func BenchmarkDagWriter(b *testing.B) {
	dstore := ds.NewNullDatastore()
	bserv, err := bs.NewBlockService(dstore, nil)
	if err != nil {
		b.Fatal(err)
	}
	dag := mdag.NewDAGService(bserv)

	b.ResetTimer()
	nbytes := int64(100000)
	for i := 0; i < b.N; i++ {
		b.SetBytes(nbytes)
		dw := dagio.NewDagWriter(dag, &chunk.SizeSplitter{Size: 4096})
		n, err := io.CopyN(dw, &datasource{}, nbytes)
		if err != nil {
			b.Fatal(err)
		}
		if n != nbytes {
			b.Fatal("Incorrect copy size.")
		}
		dw.Close()
	}

}

func TestAgainstImporter(t *testing.T) {
	dstore := ds.NewMapDatastore()
	bserv, err := bs.NewBlockService(dstore, nil)
	if err != nil {
		t.Fatal(err)
	}
	dag := mdag.NewDAGService(bserv)

	nbytes := int64(1024 * 1024 * 2)

	// DagWriter
	dw := dagio.NewDagWriter(dag, &chunk.SizeSplitter{4096})
	n, err := io.CopyN(dw, &datasource{}, nbytes)
	if err != nil {
		t.Fatal(err)
	}
	if n != nbytes {
		t.Fatal("Copied incorrect amount of bytes!")
	}

	dw.Close()
	dwNode := dw.GetNode()
	dwKey, err := dwNode.Key()
	if err != nil {
		t.Fatal(err)
	}

	// DagFromFile
	rl := &io.LimitedReader{&datasource{}, nbytes}

	dffNode, err := importer.NewDagFromReaderWithSplitter(rl, &chunk.SizeSplitter{4096})
	dffKey, err := dffNode.Key()
	if err != nil {
		t.Fatal(err)
	}
	if dwKey.String() != dffKey.String() {
		t.Errorf("\nDagWriter produced     %s\n"+
			"DagFromReader produced %s",
			dwKey, dffKey)
	}
}
