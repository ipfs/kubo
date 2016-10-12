package merkledag_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"testing"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	bstest "github.com/ipfs/go-ipfs/blockservice/test"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	imp "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	. "github.com/ipfs/go-ipfs/merkledag"
	mdpb "github.com/ipfs/go-ipfs/merkledag/pb"
	dstest "github.com/ipfs/go-ipfs/merkledag/test"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	key "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"

	"context"
	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
)

func TestNode(t *testing.T) {

	n1 := NodeWithData([]byte("beep"))
	n2 := NodeWithData([]byte("boop"))
	n3 := NodeWithData([]byte("beep boop"))
	if err := n3.AddNodeLink("beep-link", n1); err != nil {
		t.Error(err)
	}
	if err := n3.AddNodeLink("boop-link", n2); err != nil {
		t.Error(err)
	}

	printn := func(name string, n *Node) {
		fmt.Println(">", name)
		fmt.Println("data:", string(n.Data()))

		fmt.Println("links:")
		for _, l := range n.Links {
			fmt.Println("-", l.Name, l.Size, l.Hash)
		}

		e, err := n.EncodeProtobuf(false)
		if err != nil {
			t.Error(err)
		} else {
			fmt.Println("encoded:", e)
		}

		h := n.Multihash()
		k := n.Key()
		if k != key.Key(h) {
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
	enc, err := n.EncodeProtobuf(true)
	if err != nil {
		t.Error("n.EncodeProtobuf(true) failed")
		return
	}

	cumSize, err := n.Size()
	if err != nil {
		t.Error("n.Size() failed")
		return
	}

	k := n.Key()

	expected := NodeStat{
		NumLinks:       len(n.Links),
		BlockSize:      len(enc),
		LinksSize:      len(enc) - len(n.Data()), // includes framing.
		DataSize:       len(n.Data()),
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

	root, err := imp.BuildDagFromReader(dagservs[0], spl)
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

	_, err = dagservs[0].Add(root)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Added file to first node.")

	c := root.Cid()

	wg := sync.WaitGroup{}
	errs := make(chan error)

	for i := 1; i < len(dagservs); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			first, err := dagservs[i].Get(ctx, c)
			if err != nil {
				errs <- err
			}
			fmt.Println("Got first node back.")

			read, err := uio.NewDagReader(ctx, first, dagservs[i])
			if err != nil {
				errs <- err
			}
			datagot, err := ioutil.ReadAll(read)
			if err != nil {
				errs <- err
			}

			if !bytes.Equal(datagot, expected) {
				errs <- errors.New("Got bad data back!")
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertCanGet(t *testing.T, ds DAGService, n *Node) {
	if _, err := ds.Get(context.Background(), n.Cid()); err != nil {
		t.Fatal(err)
	}
}

func TestCantGet(t *testing.T) {
	ds := dstest.Mock()
	a := NodeWithData([]byte("A"))

	c := a.Cid()
	_, err := ds.Get(context.Background(), c)
	if !strings.Contains(err.Error(), "not found") {
		t.Fatal("expected err not found, got: ", err)
	}
}

func TestFetchGraph(t *testing.T) {
	var dservs []DAGService
	bsis := bstest.Mocks(2)
	for _, bsi := range bsis {
		dservs = append(dservs, NewDAGService(bsi))
	}

	read := io.LimitReader(u.NewTimeSeededRand(), 1024*32)
	root, err := imp.BuildDagFromReader(dservs[0], chunk.NewSizeSplitter(read, 512))
	if err != nil {
		t.Fatal(err)
	}

	err = FetchGraph(context.TODO(), root.Cid(), dservs[1])
	if err != nil {
		t.Fatal(err)
	}

	// create an offline dagstore and ensure all blocks were fetched
	bs := bserv.New(bsis[1].Blockstore(), offline.Exchange(bsis[1].Blockstore()))

	offline_ds := NewDAGService(bs)

	err = EnumerateChildren(context.Background(), offline_ds, root.Cid(), func(_ *cid.Cid) bool { return true }, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnumerateChildren(t *testing.T) {
	bsi := bstest.Mocks(1)
	ds := NewDAGService(bsi[0])

	read := io.LimitReader(u.NewTimeSeededRand(), 1024*1024)
	root, err := imp.BuildDagFromReader(ds, chunk.NewSizeSplitter(read, 512))
	if err != nil {
		t.Fatal(err)
	}

	set := cid.NewSet()
	err = EnumerateChildren(context.Background(), ds, root.Cid(), set.Visit, false)
	if err != nil {
		t.Fatal(err)
	}

	var traverse func(n *Node)
	traverse = func(n *Node) {
		// traverse dag and check
		for _, lnk := range n.Links {
			c := cid.NewCidV0(lnk.Hash)
			if !set.Has(c) {
				t.Fatal("missing key in set! ", lnk.Hash.B58String())
			}
			child, err := ds.Get(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			traverse(child)
		}
	}

	traverse(root)
}

func TestFetchFailure(t *testing.T) {
	ds := dstest.Mock()
	ds_bad := dstest.Mock()

	top := new(Node)
	for i := 0; i < 10; i++ {
		nd := NodeWithData([]byte{byte('a' + i)})
		_, err := ds.Add(nd)
		if err != nil {
			t.Fatal(err)
		}

		err = top.AddNodeLinkClean(fmt.Sprintf("AA%d", i), nd)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 10; i++ {
		nd := NodeWithData([]byte{'f', 'a' + byte(i)})
		_, err := ds_bad.Add(nd)
		if err != nil {
			t.Fatal(err)
		}

		err = top.AddNodeLinkClean(fmt.Sprintf("BB%d", i), nd)
		if err != nil {
			t.Fatal(err)
		}
	}

	getters := GetDAG(context.Background(), ds, top)
	for i, getter := range getters {
		_, err := getter.Get(context.Background())
		if err != nil && i < 10 {
			t.Fatal(err)
		}
		if err == nil && i >= 10 {
			t.Fatal("should have failed request")
		}
	}
}

func TestUnmarshalFailure(t *testing.T) {
	badData := []byte("hello world")

	_, err := DecodeProtobuf(badData)
	if err == nil {
		t.Fatal("shouldnt succeed to parse this")
	}

	// now with a bad link
	pbn := &mdpb.PBNode{Links: []*mdpb.PBLink{{Hash: []byte("not a multihash")}}}
	badlink, err := pbn.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecodeProtobuf(badlink)
	if err == nil {
		t.Fatal("should have failed to parse node with bad link")
	}

	n := &Node{}
	n.Marshal()
}

func TestBasicAddGet(t *testing.T) {
	ds := dstest.Mock()
	nd := new(Node)

	c, err := ds.Add(nd)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ds.Get(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}

	if !nd.Cid().Equals(out.Cid()) {
		t.Fatal("output didnt match input")
	}
}
