package dagutils

import (
	"context"
	"testing"

	dag "gx/ipfs/QmRiQCJZ91B7VNmLvA6sxzDuBJGSojS3uXHHVuNr3iueNZ/go-merkledag"
	mdtest "gx/ipfs/QmRiQCJZ91B7VNmLvA6sxzDuBJGSojS3uXHHVuNr3iueNZ/go-merkledag/test"
	path "gx/ipfs/QmdMPBephdLYNESkruDX2hcDTgFYhoCt4LimWhgnomSdV2/go-path"

	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

func TestAddLink(t *testing.T) {
	ctx, context := context.WithCancel(context.Background())
	defer context()

	ds := mdtest.Mock()
	fishnode := dag.NodeWithData([]byte("fishcakes!"))

	err := ds.Add(ctx, fishnode)
	if err != nil {
		t.Fatal(err)
	}

	nd := new(dag.ProtoNode)
	nnode, err := addLink(ctx, ds, nd, "fish", fishnode)
	if err != nil {
		t.Fatal(err)
	}

	fnprime, err := nnode.GetLinkedNode(ctx, ds, "fish")
	if err != nil {
		t.Fatal(err)
	}

	fnpkey := fnprime.Cid()
	if !fnpkey.Equals(fishnode.Cid()) {
		t.Fatal("wrong child node found!")
	}
}

func assertNodeAtPath(t *testing.T, ds ipld.DAGService, root *dag.ProtoNode, pth string, exp *cid.Cid) {
	parts := path.SplitList(pth)
	cur := root
	for _, e := range parts {
		nxt, err := cur.GetLinkedProtoNode(context.Background(), ds, e)
		if err != nil {
			t.Fatal(err)
		}

		cur = nxt
	}

	curc := cur.Cid()
	if !curc.Equals(exp) {
		t.Fatal("node not as expected at end of path")
	}
}

func TestInsertNode(t *testing.T) {
	root := new(dag.ProtoNode)
	e := NewDagEditor(root, nil)

	testInsert(t, e, "a", "anodefortesting", false, "")
	testInsert(t, e, "a/b", "data", false, "")
	testInsert(t, e, "a/b/c/d/e", "blah", false, "no link by that name")
	testInsert(t, e, "a/b/c/d/e", "foo", true, "")
	testInsert(t, e, "a/b/c/d/f", "baz", true, "")
	testInsert(t, e, "a/b/c/d/f", "bar", true, "")

	testInsert(t, e, "", "bar", true, "cannot create link with no name")
	testInsert(t, e, "////", "slashes", true, "cannot create link with no name")

	c := e.GetNode().Cid()

	if c.String() != "QmZ8yeT9uD6ouJPNAYt62XffYuXBT6b4mP4obRSE9cJrSt" {
		t.Fatal("output was different than expected: ", c)
	}
}

func testInsert(t *testing.T, e *Editor, path, data string, create bool, experr string) {
	child := dag.NodeWithData([]byte(data))
	err := e.tmp.Add(context.Background(), child)
	if err != nil {
		t.Fatal(err)
	}

	var c func() *dag.ProtoNode
	if create {
		c = func() *dag.ProtoNode {
			return &dag.ProtoNode{}
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

	assertNodeAtPath(t, e.tmp, e.root, path, child.Cid())
}
