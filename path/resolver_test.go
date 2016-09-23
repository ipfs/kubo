package path_test

import (
	"fmt"
	"testing"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	dagmock "github.com/ipfs/go-ipfs/merkledag/test"
	path "github.com/ipfs/go-ipfs/path"
	util "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

func randNode() (*merkledag.Node, key.Key) {
	node := new(merkledag.Node)
	node.SetData(make([]byte, 32))
	util.NewTimeSeededRand().Read(node.Data())
	k, _ := node.Key()
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

	aKey, err := a.Key()
	if err != nil {
		t.Fatal(err)
	}

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

	key, err := node.Key()
	if err != nil {
		t.Fatal(err)
	}
	if key.String() != cKey.String() {
		t.Fatal(fmt.Errorf(
			"recursive path resolution failed for %s: %s != %s",
			p.String(), key.String(), cKey.String()))
	}
}
