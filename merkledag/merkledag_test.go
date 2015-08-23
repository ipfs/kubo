package merkledag_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"testing"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	bstest "github.com/ipfs/go-ipfs/blockservice/test"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	imp "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	. "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	u "github.com/ipfs/go-ipfs/util"
)

type dagservAndPinner struct {
	ds DAGService
	mp pin.ManualPinner
}

func getDagservAndPinner(t *testing.T) dagservAndPinner {
	db := dssync.MutexWrap(ds.NewMapDatastore())
	bs := bstore.NewBlockstore(db)
	blockserv := bserv.New(bs, offline.Exchange(bs))
	dserv := NewDAGService(blockserv)
	mpin := pin.NewPinner(db, dserv).GetManual()
	return dagservAndPinner{
		ds: dserv,
		mp: mpin,
	}
}

func TestNode(t *testing.T) {

	n1 := &Node{Data: []byte("beep")}
	n2 := &Node{Data: []byte("boop")}
	n3 := &Node{Data: []byte("beep boop")}
	if err := n3.AddNodeLink("beep-link", n1); err != nil {
		t.Error(err)
	}
	if err := n3.AddNodeLink("boop-link", n2); err != nil {
		t.Error(err)
	}

	printn := func(name string, n *Node) {
		fmt.Println(">", name)
		fmt.Println("data:", string(n.Data))

		fmt.Println("links:")
		for _, l := range n.Links {
			fmt.Println("-", l.Name, l.Size, l.Hash)
		}

		e, err := n.Encoded(false)
		if err != nil {
			t.Error(err)
		} else {
			fmt.Println("encoded:", e)
		}

		h, err := n.Multihash()
		if err != nil {
			t.Error(err)
		} else {
			fmt.Println("hash:", h)
		}

		k, err := n.Key()
		if err != nil {
			t.Error(err)
		} else if k != key.Key(h) {
			t.Error("Key is not equivalent to multihash")
		} else {
			fmt.Println("key: ", k)
		}

		SubtestNodeStat(t, n)
	}

	printn("beep", n1)
	printn("boop", n2)
	printn("beep boop", n3)
}

func SubtestNodeStat(t *testing.T, n *Node) {
	enc, err := n.Encoded(true)
	if err != nil {
		t.Error("n.Encoded(true) failed")
		return
	}

	cumSize, err := n.Size()
	if err != nil {
		t.Error("n.Size() failed")
		return
	}

	k, err := n.Key()
	if err != nil {
		t.Error("n.Key() failed")
		return
	}

	expected := NodeStat{
		NumLinks:       len(n.Links),
		BlockSize:      len(enc),
		LinksSize:      len(enc) - len(n.Data), // includes framing.
		DataSize:       len(n.Data),
		CumulativeSize: int(cumSize),
		Hash:           k.B58String(),
	}

	actual, err := n.Stat()
	if err != nil {
		t.Error("n.Stat() failed")
		return
	}

	if expected != *actual {
		t.Errorf("n.Stat incorrect.\nexpect: %s\nactual: %s", expected, actual)
	} else {
		fmt.Printf("n.Stat correct: %s\n", actual)
	}
}

type devZero struct{}

func (_ devZero) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 0
	}
	return len(b), nil
}

func TestBatchFetch(t *testing.T) {
	read := io.LimitReader(u.NewTimeSeededRand(), 1024*32)
	runBatchFetchTest(t, read)
}

func TestBatchFetchDupBlock(t *testing.T) {
	read := io.LimitReader(devZero{}, 1024*32)
	runBatchFetchTest(t, read)
}

func runBatchFetchTest(t *testing.T, read io.Reader) {
	ctx := context.Background()
	var dagservs []DAGService
	for _, bsi := range bstest.Mocks(5) {
		dagservs = append(dagservs, NewDAGService(bsi))
	}

	spl := chunk.NewSizeSplitter(read, 512)

	root, err := imp.BuildDagFromReader(dagservs[0], spl, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("finished setup.")

	dagr, err := uio.NewDagReader(ctx, root, dagservs[0])
	if err != nil {
		t.Fatal(err)
	}

	expected, err := ioutil.ReadAll(dagr)
	if err != nil {
		t.Fatal(err)
	}

	err = dagservs[0].AddRecursive(root)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Added file to first node.")

	k, err := root.Key()
	if err != nil {
		t.Fatal(err)
	}

	wg := sync.WaitGroup{}
	for i := 1; i < len(dagservs); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			first, err := dagservs[i].Get(ctx, k)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println("Got first node back.")

			read, err := uio.NewDagReader(ctx, first, dagservs[i])
			if err != nil {
				t.Fatal(err)
			}
			datagot, err := ioutil.ReadAll(read)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(datagot, expected) {
				t.Fatal("Got bad data back!")
			}
		}(i)
	}

	wg.Wait()
}

func TestRecursiveAdd(t *testing.T) {
	a := &Node{Data: []byte("A")}
	b := &Node{Data: []byte("B")}
	c := &Node{Data: []byte("C")}
	d := &Node{Data: []byte("D")}
	e := &Node{Data: []byte("E")}

	err := a.AddNodeLink("blah", b)
	if err != nil {
		t.Fatal(err)
	}

	err = b.AddNodeLink("foo", c)
	if err != nil {
		t.Fatal(err)
	}

	err = b.AddNodeLink("bar", d)
	if err != nil {
		t.Fatal(err)
	}

	err = d.AddNodeLink("baz", e)
	if err != nil {
		t.Fatal(err)
	}

	dsp := getDagservAndPinner(t)
	err = dsp.ds.AddRecursive(a)
	if err != nil {
		t.Fatal(err)
	}

	assertCanGet(t, dsp.ds, a)
	assertCanGet(t, dsp.ds, b)
	assertCanGet(t, dsp.ds, c)
	assertCanGet(t, dsp.ds, d)
	assertCanGet(t, dsp.ds, e)
}

func assertCanGet(t *testing.T, ds DAGService, n *Node) {
	k, err := n.Key()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ds.Get(context.Background(), k); err != nil {
		t.Fatal(err)
	}
}

func TestCantGet(t *testing.T) {
	dsp := getDagservAndPinner(t)
	a := &Node{Data: []byte("A")}

	k, err := a.Key()
	if err != nil {
		t.Fatal(err)
	}

	_, err = dsp.ds.Get(context.Background(), k)
	if !strings.Contains(err.Error(), "not found") {
		t.Fatal("expected err not found, got: ", err)
	}
}
