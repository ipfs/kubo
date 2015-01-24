package coreunix

import (
	"os"
	"path"
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/repo"
	"github.com/jbenet/go-ipfs/repo/config"
	"github.com/jbenet/go-ipfs/util/testutil"
)

func TestAddRecursive(t *testing.T) {
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: "Qmfoo", // required by offline node
			},
		},
		D: testutil.ThreadSafeCloserMapDatastore(),
	}
	node, err := core.NewIPFSNode(context.Background(), core.Offline(r))
	if err != nil {
		t.Fatal(err)
	}
	if k, err := AddR(node, path.Join(here, "test_data")); err != nil {
		t.Fatal(err)
	} else if k != "QmWCCga8AbTyfAQ7pTnGT6JgmRMAB3Qp8ZmTEFi5q5o8jC" {
		t.Fatal("keys do not match")
	}
}
