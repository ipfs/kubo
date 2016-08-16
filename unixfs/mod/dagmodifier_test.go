package mod

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ipfs/go-ipfs/blocks/blockstore"
	bs "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	imp "github.com/ipfs/go-ipfs/importer"
	"github.com/ipfs/go-ipfs/importer/chunk"
	h "github.com/ipfs/go-ipfs/importer/helpers"
	trickle "github.com/ipfs/go-ipfs/importer/trickle"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	"gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/sync"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func getMockDagServ(t testing.TB) mdag.DAGService {
	dstore := ds.NewMapDatastore()
	tsds := sync.MutexWrap(dstore)
	bstore := blockstore.NewBlockstore(tsds)
	bserv := bs.New(bstore, offline.Exchange(bstore))
	return mdag.NewDAGService(bserv)
}

func getMockDagServAndBstore(t testing.TB) (mdag.DAGService, blockstore.Blockstore) {
	dstore := ds.NewMapDatastore()
	tsds := sync.MutexWrap(dstore)
	bstore := blockstore.NewBlockstore(tsds)
	bserv := bs.New(bstore, offline.Exchange(bstore))
	dserv := mdag.NewDAGService(bserv)
	return dserv, bstore
}

func getNode(t testing.TB, dserv mdag.DAGService, size int64) ([]byte, *mdag.Node) {
	in := io.LimitReader(u.NewTimeSeededRand(), size)
	node, err := imp.BuildTrickleDagFromReader(dserv, sizeSplitterGen(500)(in))
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

func sizeSplitterGen(size int64) chunk.SplitterGen {
	return func(r io.Reader) chunk.Splitter {
		return chunk.NewSizeSplitter(r, size)
	}
}

func TestDagModifierBasic(t *testing.T) {
	dserv := getMockDagServ(t)
	b, n := getNode(t, dserv, 50000)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
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

	size, err := ft.DataSize(node.Data())
	if err != nil {
		t.Fatal(err)
	}

	expected := uint64(50000 + 3500 + 3000)
	if size != expected {
		t.Fatalf("Final reported size is incorrect [%d != %d]", size, expected)
	}
}

func TestMultiWrite(t *testing.T) {
	dserv := getMockDagServ(t)
	_, n := getNode(t, dserv, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
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
	dserv := getMockDagServ(t)
	_, n := getNode(t, dserv, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
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
	dserv := getMockDagServ(t)
	_, n := getNode(t, dserv, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
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
	dserv := getMockDagServ(t)
	_, n := getNode(t, dserv, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
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
	dserv := getMockDagServ(t)
	_, n := getNode(t, dserv, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
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
	dserv := getMockDagServ(t)
	b, n := getNode(t, dserv, 50000)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
	if err != nil {
		t.Fatal(err)
	}

	err = dagmod.Truncate(12345)
	if err != nil {
		t.Fatal(err)
	}
	size, err := dagmod.Size()
	if err != nil {
		t.Fatal(err)
	}

	if size != 12345 {
		t.Fatal("size was incorrect!")
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

	err = dagmod.Truncate(10)
	if err != nil {
		t.Fatal(err)
	}

	size, err = dagmod.Size()
	if err != nil {
		t.Fatal(err)
	}

	if size != 10 {
		t.Fatal("size was incorrect!")
	}

	err = dagmod.Truncate(0)
	if err != nil {
		t.Fatal(err)
	}

	size, err = dagmod.Size()
	if err != nil {
		t.Fatal(err)
	}

	if size != 0 {
		t.Fatal("size was incorrect!")
	}
}

func TestSparseWrite(t *testing.T) {
	dserv := getMockDagServ(t)
	_, n := getNode(t, dserv, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
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

func TestSeekPastEndWrite(t *testing.T) {
	dserv := getMockDagServ(t)
	_, n := getNode(t, dserv, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 5000)
	u.NewTimeSeededRand().Read(buf[2500:])

	nseek, err := dagmod.Seek(2500, os.SEEK_SET)
	if err != nil {
		t.Fatal(err)
	}

	if nseek != 2500 {
		t.Fatal("failed to seek")
	}

	wrote, err := dagmod.Write(buf[2500:])
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

func TestRelativeSeek(t *testing.T) {
	dserv := getMockDagServ(t)
	_, n := getNode(t, dserv, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 64; i++ {
		dagmod.Write([]byte{byte(i)})
		if _, err := dagmod.Seek(1, os.SEEK_CUR); err != nil {
			t.Fatal(err)
		}
	}

	out, err := ioutil.ReadAll(dagmod)
	if err != nil {
		t.Fatal(err)
	}

	for i, v := range out {
		if v != 0 && i/2 != int(v) {
			t.Errorf("expected %d, at index %d, got %d", i/2, i, v)
		}
	}
}

func TestInvalidSeek(t *testing.T) {
	dserv := getMockDagServ(t)

	_, n := getNode(t, dserv, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
	if err != nil {
		t.Fatal(err)
	}
	_, err = dagmod.Seek(10, -10)

	if err != ErrUnrecognizedWhence {
		t.Fatal(err)
	}
}

func TestEndSeek(t *testing.T) {
	dserv := getMockDagServ(t)

	_, n := getNode(t, dserv, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
	if err != nil {
		t.Fatal(err)
	}

	_, err = dagmod.Write(make([]byte, 100))
	if err != nil {
		t.Fatal(err)
	}

	offset, err := dagmod.Seek(0, os.SEEK_CUR)
	if offset != 100 {
		t.Fatal("expected the relative seek 0 to return current location")
	}

	offset, err = dagmod.Seek(0, os.SEEK_SET)
	if offset != 0 {
		t.Fatal("expected the absolute seek to set offset at 0")
	}

	offset, err = dagmod.Seek(0, os.SEEK_END)
	if offset != 100 {
		t.Fatal("expected the end seek to set offset at end")
	}
}

func TestReadAndSeek(t *testing.T) {
	dserv := getMockDagServ(t)

	_, n := getNode(t, dserv, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
	if err != nil {
		t.Fatal(err)
	}

	writeBuf := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	dagmod.Write(writeBuf)

	if !dagmod.HasChanges() {
		t.Fatal("there are changes, this should be true")
	}

	readBuf := make([]byte, 4)
	offset, err := dagmod.Seek(0, os.SEEK_SET)
	if offset != 0 {
		t.Fatal("expected offset to be 0")
	}
	if err != nil {
		t.Fatal(err)
	}

	// read 0,1,2,3
	c, err := dagmod.Read(readBuf)
	if err != nil {
		t.Fatal(err)
	}
	if c != 4 {
		t.Fatalf("expected length of 4 got %d", c)
	}

	for i := byte(0); i < 4; i++ {
		if readBuf[i] != i {
			t.Fatalf("wrong value %d [at index %d]", readBuf[i], i)
		}
	}

	// skip 4
	_, err = dagmod.Seek(1, os.SEEK_CUR)
	if err != nil {
		t.Fatalf("error: %s, offset %d, reader offset %d", err, dagmod.curWrOff, dagmod.read.Offset())
	}

	//read 5,6,7
	readBuf = make([]byte, 3)
	c, err = dagmod.Read(readBuf)
	if err != nil {
		t.Fatal(err)
	}
	if c != 3 {
		t.Fatalf("expected length of 3 got %d", c)
	}

	for i := byte(0); i < 3; i++ {
		if readBuf[i] != i+5 {
			t.Fatalf("wrong value %d [at index %d]", readBuf[i], i)
		}

	}

}

func TestCtxRead(t *testing.T) {
	dserv := getMockDagServ(t)

	_, n := getNode(t, dserv, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
	if err != nil {
		t.Fatal(err)
	}

	_, err = dagmod.Write([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	if err != nil {
		t.Fatal(err)
	}
	dagmod.Seek(0, os.SEEK_SET)

	readBuf := make([]byte, 4)
	_, err = dagmod.CtxReadFull(ctx, readBuf)
	if err != nil {
		t.Fatal(err)
	}
	err = arrComp(readBuf, []byte{0, 1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	// TODO(Kubuxu): context cancel case, I will do it after I figure out dagreader tests,
	// because this is exacelly the same.
}

func BenchmarkDagmodWrite(b *testing.B) {
	b.StopTimer()
	dserv := getMockDagServ(b)
	_, n := getNode(b, dserv, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wrsize := 4096

	dagmod, err := NewDagModifier(ctx, n, dserv, sizeSplitterGen(512))
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
	pbd, err := ft.FromBytes(nd.Data())
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
