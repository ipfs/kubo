package path_test

import (
	"fmt"
	"testing"

	datastore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	sync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	blockstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	blockservice "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	util "github.com/ipfs/go-ipfs/util"
)

func randNode() (*merkledag.Node, util.Key) {
	node := new(merkledag.Node)
	node.Data = make([]byte, 32)
	util.NewTimeSeededRand().Read(node.Data)
	k, _ := node.Key()
	return node, k
}

func TestRecurivePathResolution(t *testing.T) {
	ctx := context.Background()
	dstore := sync.MutexWrap(datastore.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv, err := blockservice.New(bstore, offline.Exchange(bstore))
	if err != nil {
		t.Fatal(err)
	}

	dagService := merkledag.NewDAGService(bserv)

	a, _ := randNode()
	b, _ := randNode()
	c, cKey := randNode()

	err = b.AddNodeLink("grandchild", c)
	if err != nil {
		t.Fatal(err)
	}

	err = a.AddNodeLink("child", b)
	if err != nil {
		t.Fatal(err)
	}

	err = dagService.AddRecursive(a)
	if err != nil {
		t.Fatal(err)
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
