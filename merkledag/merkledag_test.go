package merkledag_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	bstest "github.com/ipfs/go-ipfs/blockservice/test"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	imp "github.com/ipfs/go-ipfs/importer"
	. "github.com/ipfs/go-ipfs/merkledag"
	mdpb "github.com/ipfs/go-ipfs/merkledag/pb"
	dstest "github.com/ipfs/go-ipfs/merkledag/test"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	chunker "gx/ipfs/QmWo8jYc19ppG7YoTsrr2kEtLRbARTJho5oNXFTR6B7Peq/go-ipfs-chunker"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
	blocks "gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
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

	printn := func(name string, n *ProtoNode) {
		fmt.Println(">", name)
		fmt.Println("data:", string(n.Data()))

		fmt.Println("links:")
		for _, l := range n.Links() {
			fmt.Println("-", l.Name, l.Size, l.Cid)
		}

		e, err := n.EncodeProtobuf(false)
		if err != nil {
			t.Error(err)
		} else {
			fmt.Println("encoded:", e)
		}

		h := n.Multihash()
		k := n.Cid().Hash()
		if k.String() != h.String() {
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

func SubtestNodeStat(t *testing.T, n *ProtoNode) {
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

	k := n.Cid()

	expected := ipld.NodeStat{
		NumLinks:       len(n.Links()),
		BlockSize:      len(enc),
		LinksSize:      len(enc) - len(n.Data()), // includes framing.
		DataSize:       len(n.Data()),
		CumulativeSize: int(cumSize),
		Hash:           k.String(),
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
	var dagservs []ipld.DAGService
	for _, bsi := range bstest.Mocks(5) {
		dagservs = append(dagservs, NewDAGService(bsi))
	}

	spl := chunker.NewSizeSplitter(read, 512)

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

	err = dagservs[0].Add(ctx, root)
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

			firstpb, ok := first.(*ProtoNode)
			if !ok {
				errs <- ErrNotProtobuf
			}

			read, err := uio.NewDagReader(ctx, firstpb, dagservs[i])
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
	var dservs []ipld.DAGService
	bsis := bstest.Mocks(2)
	for _, bsi := range bsis {
		dservs = append(dservs, NewDAGService(bsi))
	}

	read := io.LimitReader(u.NewTimeSeededRand(), 1024*32)
	root, err := imp.BuildDagFromReader(dservs[0], chunker.NewSizeSplitter(read, 512))
	if err != nil {
		t.Fatal(err)
	}

	err = FetchGraph(context.TODO(), root.Cid(), dservs[1])
	if err != nil {
		t.Fatal(err)
	}

	// create an offline dagstore and ensure all blocks were fetched
	bs := bserv.New(bsis[1].Blockstore(), offline.Exchange(bsis[1].Blockstore()))

	offlineDS := NewDAGService(bs)

	err = EnumerateChildren(context.Background(), offlineDS.GetLinks, root.Cid(), func(_ *cid.Cid) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnumerateChildren(t *testing.T) {
	bsi := bstest.Mocks(1)
	ds := NewDAGService(bsi[0])

	read := io.LimitReader(u.NewTimeSeededRand(), 1024*1024)
	root, err := imp.BuildDagFromReader(ds, chunker.NewSizeSplitter(read, 512))
	if err != nil {
		t.Fatal(err)
	}

	set := cid.NewSet()

	err = EnumerateChildren(context.Background(), ds.GetLinks, root.Cid(), set.Visit)
	if err != nil {
		t.Fatal(err)
	}

	var traverse func(n ipld.Node)
	traverse = func(n ipld.Node) {
		// traverse dag and check
		for _, lnk := range n.Links() {
			c := lnk.Cid
			if !set.Has(c) {
				t.Fatal("missing key in set! ", lnk.Cid.String())
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
	ctx := context.Background()

	ds := dstest.Mock()
	ds_bad := dstest.Mock()

	top := new(ProtoNode)
	for i := 0; i < 10; i++ {
		nd := NodeWithData([]byte{byte('a' + i)})
		err := ds.Add(ctx, nd)
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
		err := ds_bad.Add(ctx, nd)
		if err != nil {
			t.Fatal(err)
		}

		err = top.AddNodeLinkClean(fmt.Sprintf("BB%d", i), nd)
		if err != nil {
			t.Fatal(err)
		}
	}

	getters := ipld.GetDAG(ctx, ds, top)
	for i, getter := range getters {
		_, err := getter.Get(ctx)
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

	n := &ProtoNode{}
	n.Marshal()
}

func TestBasicAddGet(t *testing.T) {
	ctx := context.Background()

	ds := dstest.Mock()
	nd := new(ProtoNode)

	err := ds.Add(ctx, nd)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ds.Get(ctx, nd.Cid())
	if err != nil {
		t.Fatal(err)
	}

	if !nd.Cid().Equals(out.Cid()) {
		t.Fatal("output didnt match input")
	}
}

func TestGetRawNodes(t *testing.T) {
	ctx := context.Background()

	rn := NewRawNode([]byte("test"))

	ds := dstest.Mock()

	err := ds.Add(ctx, rn)
	if err != nil {
		t.Fatal(err)
	}

	if !rn.Cid().Equals(rn.Cid()) {
		t.Fatal("output cids didnt match")
	}

	out, err := ds.Get(ctx, rn.Cid())
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out.RawData(), []byte("test")) {
		t.Fatal("raw block should match input data")
	}

	if out.Links() != nil {
		t.Fatal("raw blocks shouldnt have links")
	}

	if out.Tree("", -1) != nil {
		t.Fatal("tree should return no paths in a raw block")
	}

	size, err := out.Size()
	if err != nil {
		t.Fatal(err)
	}
	if size != 4 {
		t.Fatal("expected size to be 4")
	}

	ns, err := out.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if ns.DataSize != 4 {
		t.Fatal("expected size to be 4, got: ", ns.DataSize)
	}

	_, _, err = out.Resolve([]string{"foo"})
	if err != ErrLinkNotFound {
		t.Fatal("shouldnt find links under raw blocks")
	}
}

func TestProtoNodeResolve(t *testing.T) {

	nd := new(ProtoNode)
	nd.SetLinks([]*ipld.Link{{Name: "foo"}})

	lnk, left, err := nd.ResolveLink([]string{"foo", "bar"})
	if err != nil {
		t.Fatal(err)
	}

	if len(left) != 1 || left[0] != "bar" {
		t.Fatal("expected the single path element 'bar' to remain")
	}

	if lnk.Name != "foo" {
		t.Fatal("how did we get anything else?")
	}

	tvals := nd.Tree("", -1)
	if len(tvals) != 1 || tvals[0] != "foo" {
		t.Fatal("expected tree to return []{\"foo\"}")
	}
}

func TestCidRetention(t *testing.T) {
	ctx := context.Background()

	nd := new(ProtoNode)
	nd.SetData([]byte("fooooo"))

	pref := nd.Cid().Prefix()
	pref.Version = 1

	c2, err := pref.Sum(nd.RawData())
	if err != nil {
		t.Fatal(err)
	}

	blk, err := blocks.NewBlockWithCid(nd.RawData(), c2)
	if err != nil {
		t.Fatal(err)
	}

	bs := dstest.Bserv()
	err = bs.AddBlock(blk)
	if err != nil {
		t.Fatal(err)
	}

	ds := NewDAGService(bs)
	out, err := ds.Get(ctx, c2)
	if err != nil {
		t.Fatal(err)
	}

	if !out.Cid().Equals(c2) {
		t.Fatal("output cid didnt match")
	}
}

func TestCidRawDoesnNeedData(t *testing.T) {
	srv := NewDAGService(dstest.Bserv())
	nd := NewRawNode([]byte("somedata"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// there is no data for this node in the blockservice
	// so dag service can't load it
	links, err := srv.GetLinks(ctx, nd.Cid())
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 0 {
		t.Fatal("raw node shouldn't have any links")
	}
}

func TestEnumerateAsyncFailsNotFound(t *testing.T) {
	ctx := context.Background()

	a := NodeWithData([]byte("foo1"))
	b := NodeWithData([]byte("foo2"))
	c := NodeWithData([]byte("foo3"))
	d := NodeWithData([]byte("foo4"))

	ds := dstest.Mock()
	for _, n := range []ipld.Node{a, b, c} {
		err := ds.Add(ctx, n)
		if err != nil {
			t.Fatal(err)
		}
	}

	parent := new(ProtoNode)
	if err := parent.AddNodeLinkClean("a", a); err != nil {
		t.Fatal(err)
	}

	if err := parent.AddNodeLinkClean("b", b); err != nil {
		t.Fatal(err)
	}

	if err := parent.AddNodeLinkClean("c", c); err != nil {
		t.Fatal(err)
	}

	if err := parent.AddNodeLinkClean("d", d); err != nil {
		t.Fatal(err)
	}

	err := ds.Add(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}

	cset := cid.NewSet()
	err = EnumerateChildrenAsync(ctx, GetLinksDirect(ds), parent.Cid(), cset.Visit)
	if err == nil {
		t.Fatal("this should have failed")
	}
}

func TestProgressIndicator(t *testing.T) {
	testProgressIndicator(t, 5)
}

func TestProgressIndicatorNoChildren(t *testing.T) {
	testProgressIndicator(t, 0)
}

func testProgressIndicator(t *testing.T, depth int) {
	ds := dstest.Mock()

	top, numChildren := mkDag(ds, depth)

	v := new(ProgressTracker)
	ctx := v.DeriveContext(context.Background())

	err := FetchGraph(ctx, top, ds)
	if err != nil {
		t.Fatal(err)
	}

	if v.Value() != numChildren+1 {
		t.Errorf("wrong number of children reported in progress indicator, expected %d, got %d",
			numChildren+1, v.Value())
	}
}

func mkDag(ds ipld.DAGService, depth int) (*cid.Cid, int) {
	ctx := context.Background()

	totalChildren := 0
	f := func() *ProtoNode {
		p := new(ProtoNode)
		buf := make([]byte, 16)
		rand.Read(buf)

		p.SetData(buf)
		err := ds.Add(ctx, p)
		if err != nil {
			panic(err)
		}
		return p
	}

	for i := 0; i < depth; i++ {
		thisf := f
		f = func() *ProtoNode {
			pn := mkNodeWithChildren(thisf, 10)
			err := ds.Add(ctx, pn)
			if err != nil {
				panic(err)
			}
			totalChildren += 10
			return pn
		}
	}

	nd := f()
	err := ds.Add(ctx, nd)
	if err != nil {
		panic(err)
	}

	return nd.Cid(), totalChildren
}

func mkNodeWithChildren(getChild func() *ProtoNode, width int) *ProtoNode {
	cur := new(ProtoNode)

	for i := 0; i < width; i++ {
		c := getChild()
		if err := cur.AddNodeLinkClean(fmt.Sprint(i), c); err != nil {
			panic(err)
		}
	}

	return cur
}
