package merkledag

import (
	"testing"
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
