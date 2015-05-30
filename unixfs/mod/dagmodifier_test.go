package mod

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	bs "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	imp "github.com/ipfs/go-ipfs/importer"
	"github.com/ipfs/go-ipfs/importer/chunk"
	h "github.com/ipfs/go-ipfs/importer/helpers"
	trickle "github.com/ipfs/go-ipfs/importer/trickle"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	u "github.com/ipfs/go-ipfs/util"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

func getMockDagServ(t testing.TB) (mdag.DAGService, pin.ManualPinner) {
	dstore := ds.NewMapDatastore()
	tsds := sync.MutexWrap(dstore)
	bstore := blockstore.NewBlockstore(tsds)
	bserv, err := bs.New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Fatal(err)
	}
	dserv := mdag.NewDAGService(bserv)
	return dserv, pin.NewPinner(tsds, dserv).GetManual()
}

func getMockDagServAndBstore(t testing.TB) (mdag.DAGService, blockstore.Blockstore, pin.ManualPinner) {
	dstore := ds.NewMapDatastore()
	tsds := sync.MutexWrap(dstore)
	bstore := blockstore.NewBlockstore(tsds)
	bserv, err := bs.New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Fatal(err)
	}
	dserv := mdag.NewDAGService(bserv)
	return dserv, bstore, pin.NewPinner(tsds, dserv).GetManual()
}

func getNode(t testing.TB, dserv mdag.DAGService, size int64, pinner pin.ManualPinner) ([]byte, *mdag.Node) {
	in := io.LimitReader(u.NewTimeSeededRand(), size)
	node, err := imp.BuildTrickleDagFromReader(in, dserv, &chunk.SizeSplitter{500}, imp.BasicPinnerCB(pinner))
	if err != nil {
		t.Fatal(err)
	}

	dr, err := uio.NewDagReader(context.Background(), node, dserv)
	if err != nil {
		t.Fatal(err)
	}

	b, err := ioutil.ReadAll(dr)
	if err != nil {
		t.Fatal(err)
	}

	return b, node
}

func testModWrite(t *testing.T, beg, size uint64, orig []byte, dm *DagModifier) []byte {
	newdata := make([]byte, size)
	r := u.NewTimeSeededRand()
	r.Read(newdata)

	if size+beg > uint64(len(orig)) {
		orig = append(orig, make([]byte, (size+beg)-uint64(len(orig)))...)
	}
	copy(orig[beg:], newdata)

	nmod, err := dm.WriteAt(newdata, int64(beg))
	if err != nil {
		t.Fatal(err)
	}

	if nmod != int(size) {
		t.Fatalf("Mod length not correct! %d != %d", nmod, size)
	}

	nd, err := dm.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	err = trickle.VerifyTrickleDagStructure(nd, dm.dagserv, h.DefaultLinksPerBlock, 4)
	if err != nil {
		t.Fatal(err)
	}

	rd, err := uio.NewDagReader(context.Background(), nd, dm.dagserv)
	if err != nil {
		t.Fatal(err)
	}

	after, err := ioutil.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(after, orig)
	if err != nil {
		t.Fatal(err)
	}
	return orig
}

func TestDagModifierBasic(t *testing.T) {
	dserv, pin := getMockDagServ(t)
	b, n := getNode(t, dserv, 50000, pin)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, pin, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		t.Fatal(err)
	}

	// Within zero block
	beg := uint64(15)
	length := uint64(60)

	t.Log("Testing mod within zero block")
	b = testModWrite(t, beg, length, b, dagmod)

	// Within bounds of existing file
	beg = 1000
	length = 4000
	t.Log("Testing mod within bounds of existing multiblock file.")
	b = testModWrite(t, beg, length, b, dagmod)

	// Extend bounds
	beg = 49500
	length = 4000

	t.Log("Testing mod that extends file.")
	b = testModWrite(t, beg, length, b, dagmod)

	// "Append"
	beg = uint64(len(b))
	length = 3000
	t.Log("Testing pure append")
	b = testModWrite(t, beg, length, b, dagmod)

	// Verify reported length
	node, err := dagmod.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	size, err := ft.DataSize(node.Data)
	if err != nil {
		t.Fatal(err)
	}

	expected := uint64(50000 + 3500 + 3000)
	if size != expected {
		t.Fatalf("Final reported size is incorrect [%d != %d]", size, expected)
	}
}

func TestMultiWrite(t *testing.T) {
	dserv, pins := getMockDagServ(t)
	_, n := getNode(t, dserv, 0, pins)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, pins, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 4000)
	u.NewTimeSeededRand().Read(data)

	for i := 0; i < len(data); i++ {
		n, err := dagmod.WriteAt(data[i:i+1], int64(i))
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatal("Somehow wrote the wrong number of bytes! (n != 1)")
		}

		size, err := dagmod.Size()
		if err != nil {
			t.Fatal(err)
		}

		if size != int64(i+1) {
			t.Fatal("Size was reported incorrectly")
		}
	}
	nd, err := dagmod.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	read, err := uio.NewDagReader(context.Background(), nd, dserv)
	if err != nil {
		t.Fatal(err)
	}
	rbuf, err := ioutil.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(rbuf, data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultiWriteAndFlush(t *testing.T) {
	dserv, pins := getMockDagServ(t)
	_, n := getNode(t, dserv, 0, pins)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, pins, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 20)
	u.NewTimeSeededRand().Read(data)

	for i := 0; i < len(data); i++ {
		n, err := dagmod.WriteAt(data[i:i+1], int64(i))
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatal("Somehow wrote the wrong number of bytes! (n != 1)")
		}
		err = dagmod.Sync()
		if err != nil {
			t.Fatal(err)
		}
	}
	nd, err := dagmod.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	read, err := uio.NewDagReader(context.Background(), nd, dserv)
	if err != nil {
		t.Fatal(err)
	}
	rbuf, err := ioutil.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(rbuf, data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteNewFile(t *testing.T) {
	dserv, pins := getMockDagServ(t)
	_, n := getNode(t, dserv, 0, pins)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, pins, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		t.Fatal(err)
	}

	towrite := make([]byte, 2000)
	u.NewTimeSeededRand().Read(towrite)

	nw, err := dagmod.Write(towrite)
	if err != nil {
		t.Fatal(err)
	}
	if nw != len(towrite) {
		t.Fatal("Wrote wrong amount")
	}

	nd, err := dagmod.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	read, err := uio.NewDagReader(ctx, nd, dserv)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	if err := arrComp(data, towrite); err != nil {
		t.Fatal(err)
	}
}

func TestMultiWriteCoal(t *testing.T) {
	dserv, pins := getMockDagServ(t)
	_, n := getNode(t, dserv, 0, pins)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, pins, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 1000)
	u.NewTimeSeededRand().Read(data)

	for i := 0; i < len(data); i++ {
		n, err := dagmod.WriteAt(data[:i+1], 0)
		if err != nil {
			fmt.Println("FAIL AT ", i)
			t.Fatal(err)
		}
		if n != i+1 {
			t.Fatal("Somehow wrote the wrong number of bytes! (n != 1)")
		}

	}
	nd, err := dagmod.GetNode()
	if err != nil {
		t.Fatal(err)
	}

	read, err := uio.NewDagReader(context.Background(), nd, dserv)
	if err != nil {
		t.Fatal(err)
	}
	rbuf, err := ioutil.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	err = arrComp(rbuf, data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLargeWriteChunks(t *testing.T) {
	dserv, pins := getMockDagServ(t)
	_, n := getNode(t, dserv, 0, pins)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, pins, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		t.Fatal(err)
	}

	wrsize := 1000
	datasize := 10000000
	data := make([]byte, datasize)

	u.NewTimeSeededRand().Read(data)

	for i := 0; i < datasize/wrsize; i++ {
		n, err := dagmod.WriteAt(data[i*wrsize:(i+1)*wrsize], int64(i*wrsize))
		if err != nil {
			t.Fatal(err)
		}
		if n != wrsize {
			t.Fatal("failed to write buffer")
		}
	}

	out, err := ioutil.ReadAll(dagmod)
	if err != nil {
		t.Fatal(err)
	}

	if err = arrComp(out, data); err != nil {
		t.Fatal(err)
	}

}

func TestDagTruncate(t *testing.T) {
	dserv, pins := getMockDagServ(t)
	b, n := getNode(t, dserv, 50000, pins)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, pins, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		t.Fatal(err)
	}

	err = dagmod.Truncate(12345)
	if err != nil {
		t.Fatal(err)
	}

	_, err = dagmod.Seek(0, os.SEEK_SET)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(dagmod)
	if err != nil {
		t.Fatal(err)
	}

	if err = arrComp(out, b[:12345]); err != nil {
		t.Fatal(err)
	}
}

func TestSparseWrite(t *testing.T) {
	dserv, pins := getMockDagServ(t)
	_, n := getNode(t, dserv, 0, pins)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, pins, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 5000)
	u.NewTimeSeededRand().Read(buf[2500:])

	wrote, err := dagmod.WriteAt(buf[2500:], 2500)
	if err != nil {
		t.Fatal(err)
	}

	if wrote != 2500 {
		t.Fatal("incorrect write amount")
	}

	_, err = dagmod.Seek(0, os.SEEK_SET)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(dagmod)
	if err != nil {
		t.Fatal(err)
	}

	if err = arrComp(out, buf); err != nil {
		t.Fatal(err)
	}
}

func basicGC(t *testing.T, bs blockstore.Blockstore, pins pin.ManualPinner) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // in case error occurs during operation
	keychan, err := bs.AllKeysChan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for k := range keychan { // rely on AllKeysChan to close chan
		if !pins.IsPinned(k) {
			err := bs.DeleteBlock(k)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestCorrectPinning(t *testing.T) {
	dserv, bstore, pins := getMockDagServAndBstore(t)
	b, n := getNode(t, dserv, 50000, pins)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, pins, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1024)
	for i := 0; i < 100; i++ {
		size, err := dagmod.Size()
		if err != nil {
			t.Fatal(err)
		}
		offset := rand.Intn(int(size))
		u.NewTimeSeededRand().Read(buf)

		if offset+len(buf) > int(size) {
			b = append(b[:offset], buf...)
		} else {
			copy(b[offset:], buf)
		}

		n, err := dagmod.WriteAt(buf, int64(offset))
		if err != nil {
			t.Fatal(err)
		}
		if n != len(buf) {
			t.Fatal("wrote incorrect number of bytes")
		}
	}

	fisize, err := dagmod.Size()
	if err != nil {
		t.Fatal(err)
	}

	if int(fisize) != len(b) {
		t.Fatal("reported filesize incorrect", fisize, len(b))
	}

	// Run a GC, then ensure we can still read the file correctly
	basicGC(t, bstore, pins)

	nd, err := dagmod.GetNode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := uio.NewDagReader(context.Background(), nd, dserv)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	if err = arrComp(out, b); err != nil {
		t.Fatal(err)
	}

	rootk, err := nd.Key()
	if err != nil {
		t.Fatal(err)
	}

	// Verify only one recursive pin
	recpins := pins.RecursiveKeys()
	if len(recpins) != 1 {
		t.Fatal("Incorrect number of pinned entries")
	}

	// verify the correct node is pinned
	if recpins[0] != rootk {
		t.Fatal("Incorrect node recursively pinned")
	}

	indirpins := pins.IndirectKeys()
	children := enumerateChildren(t, nd, dserv)
	if len(indirpins) != len(children) {
		t.Log(len(indirpins), len(children))
		t.Fatal("Incorrect number of indirectly pinned blocks")
	}

}

func enumerateChildren(t *testing.T, nd *mdag.Node, ds mdag.DAGService) []u.Key {
	var out []u.Key
	for _, lnk := range nd.Links {
		out = append(out, u.Key(lnk.Hash))
		child, err := lnk.GetNode(context.Background(), ds)
		if err != nil {
			t.Fatal(err)
		}
		children := enumerateChildren(t, child, ds)
		out = append(out, children...)
	}
	return out
}

func BenchmarkDagmodWrite(b *testing.B) {
	b.StopTimer()
	dserv, pins := getMockDagServ(b)
	_, n := getNode(b, dserv, 0, pins)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wrsize := 4096

	dagmod, err := NewDagModifier(ctx, n, dserv, pins, &chunk.SizeSplitter{Size: 512})
	if err != nil {
		b.Fatal(err)
	}

	buf := make([]byte, b.N*wrsize)
	u.NewTimeSeededRand().Read(buf)
	b.StartTimer()
	b.SetBytes(int64(wrsize))
	for i := 0; i < b.N; i++ {
		n, err := dagmod.Write(buf[i*wrsize : (i+1)*wrsize])
		if err != nil {
			b.Fatal(err)
		}
		if n != wrsize {
			b.Fatal("Wrote bad size")
		}
	}
}

func arrComp(a, b []byte) error {
	if len(a) != len(b) {
		return fmt.Errorf("Arrays differ in length. %d != %d", len(a), len(b))
	}
	for i, v := range a {
		if v != b[i] {
			return fmt.Errorf("Arrays differ at index: %d", i)
		}
	}
	return nil
}

func printDag(nd *mdag.Node, ds mdag.DAGService, indent int) {
	pbd, err := ft.FromBytes(nd.Data)
	if err != nil {
		panic(err)
	}

	for i := 0; i < indent; i++ {
		fmt.Print(" ")
	}
	fmt.Printf("{size = %d, type = %s, children = %d", pbd.GetFilesize(), pbd.GetType().String(), len(pbd.GetBlocksizes()))
	if len(nd.Links) > 0 {
		fmt.Println()
	}
	for _, lnk := range nd.Links {
		child, err := lnk.GetNode(context.Background(), ds)
		if err != nil {
			panic(err)
		}
		printDag(child, ds, indent+1)
	}
	if len(nd.Links) > 0 {
		for i := 0; i < indent; i++ {
			fmt.Print(" ")
		}
	}
	fmt.Println("}")
}
