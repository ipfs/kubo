package dagutils

import (
	"testing"

	key "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mdtest "github.com/ipfs/go-ipfs/merkledag/test"
	path "github.com/ipfs/go-ipfs/path"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func TestAddLink(t *testing.T) {
	ds := mdtest.Mock()
	fishnode := dag.NodeWithData([]byte("fishcakes!"))

	fk, err := ds.Add(fishnode)
	if err != nil {
		t.Fatal(err)
	}

	nd := new(dag.Node)
	nnode, err := addLink(context.Background(), ds, nd, "fish", fishnode)
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

func assertNodeAtPath(t *testing.T, ds dag.DAGService, root *dag.Node, pth string, exp key.Key) {
	parts := path.SplitList(pth)
	cur := root
	for _, e := range parts {
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
	root := new(dag.Node)
	e := NewDagEditor(root, nil)

	testInsert(t, e, "a", "anodefortesting", false, "")
	testInsert(t, e, "a/b", "data", false, "")
	testInsert(t, e, "a/b/c/d/e", "blah", false, "no link by that name")
	testInsert(t, e, "a/b/c/d/e", "foo", true, "")
	testInsert(t, e, "a/b/c/d/f", "baz", true, "")
	testInsert(t, e, "a/b/c/d/f", "bar", true, "")

	testInsert(t, e, "", "bar", true, "cannot create link with no name!")
	testInsert(t, e, "////", "slashes", true, "cannot create link with no name!")

	k, err := e.GetNode().Key()
	if err != nil {
		t.Fatal(err)
	}

	if k.B58String() != "QmZ8yeT9uD6ouJPNAYt62XffYuXBT6b4mP4obRSE9cJrSt" {
		t.Fatal("output was different than expected: ", k)
	}
}

func testInsert(t *testing.T, e *Editor, path, data string, create bool, experr string) {
	child := dag.NodeWithData([]byte(data))
	ck, err := e.tmp.Add(child)
	if err != nil {
		t.Fatal(err)
	}

	var c func() *dag.Node
	if create {
		c = func() *dag.Node {
			return &dag.Node{}
		}
	}

	err = e.InsertNodeAtPath(context.Background(), path, child, c)
	if experr != "" {
		var got string
		if err != nil {
			got = err.Error()
		}
		if got != experr {
			t.Fatalf("expected '%s' but got '%s'", experr, got)
		}
		return
	}

	if err != nil {
		t.Fatal(err, path, data, create, experr)
	}

	assertNodeAtPath(t, e.tmp, e.root, path, ck)
}
