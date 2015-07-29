package corerepo

import (
	"io"
	"testing"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	importer "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	pin "github.com/ipfs/go-ipfs/pin"
	u "github.com/ipfs/go-ipfs/util"
)

func TestBasicSeparation(t *testing.T) {
	nd, err := core.NewNode(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	ks := make(map[key.Key]struct{})
	for i := 0; i < 5; i++ {
		k := randomDag(t, nd.StateDAG)
		ks[k] = struct{}{}
	}

	for k, _ := range ks {
		_, err := nd.DAG.Get(context.TODO(), k)
		if err == nil {
			t.Fatal("shouldnt have found this here")
		}
	}

	kch, err := nd.DataBlocks.AllKeysChan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for k := range kch {
		if _, ok := ks[k]; ok {
			t.Fatal("bad")
		}
	}
}

func TestGC(t *testing.T) {
	nd, err := core.NewNode(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	var pubdagk []key.Key
	for i := 0; i < 50; i++ {
		pubdagk = append(pubdagk, randomDag(t, nd.DAG))
	}

	var statedagk []key.Key
	for i := 0; i < 50; i++ {
		statedagk = append(statedagk, randomDag(t, nd.StateDAG))
	}

	// pin the first 10 keys from the pubdagk list
	for _, k := range pubdagk[:10] {
		_, err := Pin(nd, context.Background(), []string{k.B58String()}, true)
		if err != nil {
			t.Fatal(err)
		}
	}

	// before we garbage collect, lets make sure no pinning blocks leaked into our
	// data blockstore
	for _, k := range nd.Pinning.InternalPins() {
		assertNotFound(t, k, nd.DataBlocks)
	}

	err = GarbageCollect(nd, context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// now check to make sure things are where we expect them
	for _, k := range pubdagk[:10] {
		assertDagLocal(t, k, nd.DAG)
	}

	for _, k := range nd.Pinning.InternalPins() {
		assertBlocksLocal(t, k, nd.StateBlocks)
	}

	for _, k := range pubdagk[10:] {
		assertNotFound(t, k, nd.DataBlocks)
	}

	// a subsequent GC shouldnt remove anything
	ch, err := GarbageCollectAsync(nd, context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _ = range ch {
		t.Fatal("shouldnt have removed anything")
	}

	// even after reloading the pinner, GC shouldnt remove anything
	npin, err := pin.LoadPinner(nd.Repo.Datastore(), nd.DAG, nd.StateDAG)
	if err != nil {
		t.Fatal(err)
	}
	nd.Pinning = npin

	ch, err = GarbageCollectAsync(nd, context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _ = range ch {
		t.Fatal("shouldnt have removed anything")
	}
}

func randomDag(t *testing.T, ds dag.DAGService) key.Key {
	r := u.NewTimeSeededRand()

	r = io.LimitReader(r, 10000)

	nd, err := importer.BuildDagFromReader(ds, chunk.NewSizeSplitter(r, 512))
	if err != nil {
		t.Fatal(err)
	}

	k, err := nd.Key()
	if err != nil {
		t.Fatal(err)
	}

	return k
}

func assertDagLocal(t *testing.T, root key.Key, ds dag.DAGService) {
	rootnd, err := ds.Get(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	ks := key.NewKeySet()
	err = dag.EnumerateChildren(context.Background(), ds, rootnd, ks)
	if err != nil {
		t.Fatal(err)
	}
}

func assertBlocksLocal(t *testing.T, k key.Key, bs bstore.Blockstore) {
	_, err := bs.Get(k)
	if err != nil {
		t.Fatal("expected to find block: ", err)
	}
}

func assertNotFound(t *testing.T, k key.Key, bs bstore.Blockstore) {
	bk := []byte(k)
	hasIssue := (bk[len(bk)-1] == '/')

	_, err := bs.Get(k)
	if err != bstore.ErrNotFound {
		if hasIssue {
			// keys ending in the byte 47 (slash) wont show up in a blockstore
			// listing due to bug 1554, for now, if they show up after GC, we should
			// ignore them
			t.Log("KNOWN ERROR: please fix bug 1554")
			return
		}
		t.Fatal("expected not found:", err, k, []byte(k))
		// NOTE: may also fail for a number of reasons, example:
		// - if the key contains two consecutive '/' characters
	}

	if hasIssue {
		// if we get here, that means we fixed issue 1554 and didnt update this test
		// lets leave ourselves a reminder
		t.Fatal("if you fixed 1554, please update this code")
	}
}
