package path_test

import (
	"fmt"
	"testing"

	context "context"

	merkledag "github.com/ipfs/go-ipfs/merkledag"
	dagmock "github.com/ipfs/go-ipfs/merkledag/test"
	path "github.com/ipfs/go-ipfs/path"
	key "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"
	util "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
)

func randNode() (*merkledag.Node, key.Key) {
	node := new(merkledag.Node)
	node.SetData(make([]byte, 32))
	util.NewTimeSeededRand().Read(node.Data())
	k := node.Key()
	return node, k
}

func TestRecurivePathResolution(t *testing.T) {
	ctx := context.Background()
	dagService := dagmock.Mock()

	a, _ := randNode()
	b, _ := randNode()
	c, cKey := randNode()

	err := b.AddNodeLink("grandchild", c)
	if err != nil {
		t.Fatal(err)
	}

	err = a.AddNodeLink("child", b)
	if err != nil {
		t.Fatal(err)
	}

	for _, n := range []*merkledag.Node{a, b, c} {
		_, err = dagService.Add(n)
		if err != nil {
			t.Fatal(err)
		}
	}

	aKey := a.Key()

	segments := []string{aKey.String(), "child", "grandchild"}
	p, err := path.FromSegments("/ipfs/", segments...)
	if err != nil {
		t.Fatal(err)
	}

	resolver := &path.Resolver{DAG: dagService}
	node, err := resolver.ResolvePath(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	key := node.Key()
	if key.String() != cKey.String() {
		t.Fatal(fmt.Errorf(
			"recursive path resolution failed for %s: %s != %s",
			p.String(), key.String(), cKey.String()))
	}
}
