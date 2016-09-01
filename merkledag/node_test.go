package merkledag_test

import (
	"testing"

	. "github.com/ipfs/go-ipfs/merkledag"
	mdtest "github.com/ipfs/go-ipfs/merkledag/test"

	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func TestRemoveLink(t *testing.T) {
	nd := &Node{
		Links: []*Link{
			&Link{Name: "a"},
			&Link{Name: "b"},
			&Link{Name: "a"},
			&Link{Name: "a"},
			&Link{Name: "c"},
			&Link{Name: "a"},
		},
	}

	err := nd.RemoveNodeLink("a")
	if err != nil {
		t.Fatal(err)
	}

	if len(nd.Links) != 2 {
		t.Fatal("number of links incorrect")
	}

	if nd.Links[0].Name != "b" {
		t.Fatal("link order wrong")
	}

	if nd.Links[1].Name != "c" {
		t.Fatal("link order wrong")
	}

	// should fail
	err = nd.RemoveNodeLink("a")
	if err != ErrNotFound {
		t.Fatal("should have failed to remove link")
	}

	// ensure nothing else got touched
	if len(nd.Links) != 2 {
		t.Fatal("number of links incorrect")
	}

	if nd.Links[0].Name != "b" {
		t.Fatal("link order wrong")
	}

	if nd.Links[1].Name != "c" {
		t.Fatal("link order wrong")
	}
}

func TestFindLink(t *testing.T) {
	ds := mdtest.Mock()
	k, err := ds.Add(new(Node))
	if err != nil {
		t.Fatal(err)
	}

	nd := &Node{
		Links: []*Link{
			&Link{Name: "a", Hash: k.Hash()},
			&Link{Name: "c", Hash: k.Hash()},
			&Link{Name: "b", Hash: k.Hash()},
		},
	}

	_, err = ds.Add(nd)
	if err != nil {
		t.Fatal(err)
	}

	lnk, err := nd.GetNodeLink("b")
	if err != nil {
		t.Fatal(err)
	}

	if lnk.Name != "b" {
		t.Fatal("got wrong link back")
	}

	_, err = nd.GetNodeLink("f")
	if err != ErrLinkNotFound {
		t.Fatal("shouldnt have found link")
	}

	_, err = nd.GetLinkedNode(context.Background(), ds, "b")
	if err != nil {
		t.Fatal(err)
	}

	outnd, err := nd.UpdateNodeLink("b", nd)
	if err != nil {
		t.Fatal(err)
	}

	olnk, err := outnd.GetNodeLink("b")
	if err != nil {
		t.Fatal(err)
	}

	if olnk.Hash.B58String() == k.String() {
		t.Fatal("new link should have different hash")
	}
}

func TestNodeCopy(t *testing.T) {
	nd := &Node{
		Links: []*Link{
			&Link{Name: "a"},
			&Link{Name: "c"},
			&Link{Name: "b"},
		},
	}
	nd.SetData([]byte("testing"))

	ond := nd.Copy()
	ond.SetData(nil)

	if nd.Data() == nil {
		t.Fatal("should be different objects")
	}
}
