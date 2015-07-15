package merkledag_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
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
	mp pin.Pinner
}

func getDagservAndPinner(t *testing.T) dagservAndPinner {
	db := dssync.MutexWrap(ds.NewMapDatastore())
	bs := bstore.NewBlockstore(db)
	blockserv, err := bserv.New(bs, offline.Exchange(bs))
	if err != nil {
		t.Fatal(err)
	}
	dserv := NewDAGService(blockserv)
	mpin := pin.NewPinner(db, dserv)
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

	expected := NodeStat{
		NumLinks:       len(n.Links),
		BlockSize:      len(enc),
		LinksSize:      len(enc) - len(n.Data), // includes framing.
		DataSize:       len(n.Data),
		CumulativeSize: int(cumSize),
	}

	actual, err := n.Stat()
	if err != nil {
		t.Error("n.Stat() failed")
		return
	}

	if expected != *actual {
		t.Error("n.Stat incorrect.\nexpect: %s\nactual: %s", expected, actual)
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
	var dagservs []DAGService
	for _, bsi := range bstest.Mocks(t, 5) {
		dagservs = append(dagservs, NewDAGService(bsi))
	}

	spl := &chunk.SizeSplitter{512}

	root, err := imp.BuildDagFromReader(read, dagservs[0], spl)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("finished setup.")

	dagr, err := uio.NewDagReader(context.TODO(), root, dagservs[0])
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
			first, err := dagservs[i].Get(context.Background(), k)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println("Got first node back.")

			read, err := uio.NewDagReader(context.TODO(), first, dagservs[i])
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

func TestFetchGraph(t *testing.T) {
	var dservs []DAGService
	bsis := bstest.Mocks(t, 2)
	for _, bsi := range bsis {
		dservs = append(dservs, NewDAGService(bsi))
	}

	read := io.LimitReader(u.NewTimeSeededRand(), 1024*32)
	spl := &chunk.SizeSplitter{512}

	root, err := imp.BuildDagFromReader(read, dservs[0], spl)
	if err != nil {
		t.Fatal(err)
	}

	err = FetchGraph(context.TODO(), root, dservs[1])
	if err != nil {
		t.Fatal(err)
	}

	// create an offline dagstore and ensure all blocks were fetched
	bs, err := bserv.New(bsis[1].Blockstore, offline.Exchange(bsis[1].Blockstore))
	if err != nil {
		t.Fatal(err)
	}

	offline_ds := NewDAGService(bs)
	ks := key.NewKeySet()

	err = EnumerateChildren(context.Background(), offline_ds, root, ks)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnumerateChildren(t *testing.T) {
	bsi := bstest.Mocks(t, 1)
	ds := NewDAGService(bsi[0])

	spl := &chunk.SizeSplitter{512}

	read := io.LimitReader(u.NewTimeSeededRand(), 1024*1024)

	root, err := imp.BuildDagFromReader(read, ds, spl)
	if err != nil {
		t.Fatal(err)
	}

	ks := key.NewKeySet()
	err = EnumerateChildren(context.Background(), ds, root, ks)
	if err != nil {
		t.Fatal(err)
	}

	var traverse func(n *Node)
	traverse = func(n *Node) {
		// traverse dag and check
		for _, lnk := range n.Links {
			k := key.Key(lnk.Hash)
			if !ks.Has(k) {
				t.Fatal("missing key in set!")
			}
			child, err := ds.Get(context.Background(), k)
			if err != nil {
				t.Fatal(err)
			}
			traverse(child)
		}
	}

	traverse(root)
}
