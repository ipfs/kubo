package io

import (
	"testing"

	"io"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	bs "github.com/jbenet/go-ipfs/blockservice"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	mdag "github.com/jbenet/go-ipfs/merkledag"
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
	dag := &mdag.DAGService{bserv}
	dw := NewDagWriter(dag, &chunk.SizeSplitter{4096})

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
	read, err := NewDagReader(node, dag)
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
	dag := &mdag.DAGService{bserv}
	dw := NewDagWriter(dag, &chunk.SizeSplitter{4096})

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
	dag := &mdag.DAGService{bserv}

	b.ResetTimer()
	nbytes := int64(100000)
	for i := 0; i < b.N; i++ {
		b.SetBytes(nbytes)
		dw := NewDagWriter(dag, &chunk.SizeSplitter{4096})
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
