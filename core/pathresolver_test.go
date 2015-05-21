package core

import (
	"testing"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	config "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/util/testutil"
	"github.com/ipfs/go-ipfs/repo"
	path "github.com/ipfs/go-ipfs/path"
)

func TestResolveInvalidPath(t *testing.T) {
	ctx := context.TODO()
	id := testIdentity

	r := &repo.Mock{
		C: config.Config{
			Identity: id,
			Datastore: config.Datastore{
				Type: "memory",
			},
			Addresses: config.Addresses{
				Swarm: []string{"/ip4/0.0.0.0/tcp/4001"},
				API:   "/ip4/127.0.0.1/tcp/8000",
			},
		},
		D: testutil.ThreadSafeCloserMapDatastore(),
	}

	n, err := NewIPFSNode(ctx, Standard(r, false))
	if n == nil || err != nil {
		t.Error("Should have constructed.", err)
	}

	_, err = Resolve(ctx, n, path.Path("/ipfs/"))
	if err == nil {
		t.Error("Should get invalid path")
	}

}
