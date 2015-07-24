package dagutils

import (
	"testing"

	key "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mdtest "github.com/ipfs/go-ipfs/merkledag/test"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

func TestAddLink(t *testing.T) {
	ds := mdtest.Mock(t)
	fishnode := &dag.Node{
		Data: []byte("fishcakes!"),
	}

	fk, err := ds.Add(fishnode)
	if err != nil {
		t.Fatal(err)
	}

	nd := new(dag.Node)
	nnode, err := AddLink(context.Background(), ds, nd, "fish", fk)
	if err != nil {
		t.Fatal(err)
	}

	fnprime, err := nnode.GetLinkedNode(context.Background(), ds, "fish")
	if err != nil {
		t.Fatal(err)
	}

	fnpkey, err := fnprime.Key()
	if err != nil {
		t.Fatal(err)
	}

	if fnpkey != fk {
		t.Fatal("wrong child node found!")
	}
}

func assertNodeAtPath(t *testing.T, ds dag.DAGService, root *dag.Node, path []string, exp key.Key) {
	cur := root
	for _, e := range path {
		nxt, err := cur.GetLinkedNode(context.Background(), ds, e)
		if err != nil {
			t.Fatal(err)
		}

		cur = nxt
	}

	curk, err := cur.Key()
	if err != nil {
		t.Fatal(err)
	}

	if curk != exp {
		t.Fatal("node not as expected at end of path")
	}
}

func TestInsertNode(t *testing.T) {
	ds := mdtest.Mock(t)
	root := new(dag.Node)

	childa := &dag.Node{
		Data: []byte("This is child A"),
	}
	ak, err := ds.Add(childa)
	if err != nil {
		t.Fatal(err)
	}

	path := []string{"a", "b", "c", "d"}
	root_a, err := InsertNodeAtPath(context.Background(), ds, root, path, ak, true)
	if err != nil {
		t.Fatal(err)
	}
	assertNodeAtPath(t, ds, root_a, path, ak)

	childb := &dag.Node{Data: []byte("this is the second child")}
	bk, err := ds.Add(childb)
	if err != nil {
		t.Fatal(err)
	}

	// this one should fail, we are specifying a non-existant path
	// with create == false
	path2 := []string{"a", "b", "e", "f"}
	_, err = InsertNodeAtPath(context.Background(), ds, root_a, path2, bk, false)
	if err == nil {
		t.Fatal("that shouldnt have worked")
	}
	if err != dag.ErrNotFound {
		t.Fatal("expected this to fail with 'not found'")
	}

	// inserting a path of length one should work with create == false
	path3 := []string{"x"}
	root_b, err := InsertNodeAtPath(context.Background(), ds, root_a, path3, bk, false)
	if err != nil {
		t.Fatal(err)
	}

	assertNodeAtPath(t, ds, root_b, path3, bk)

	// now try overwriting a path
	root_c, err := InsertNodeAtPath(context.Background(), ds, root_b, path, bk, false)
	if err != nil {
		t.Fatal(err)
	}

	assertNodeAtPath(t, ds, root_c, path, bk)
}
