package test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/ipld/merkledag"
	uio "github.com/ipfs/boxo/ipld/unixfs/io"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipld/go-ipld-prime"
)

func TestPathUnixFSHAMTPartial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a node
	apis, err := NodeProvider{}.MakeAPISwarm(t, ctx, true, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	a := apis[0]

	// Setting this after instantiating the swarm so that it's not clobbered by loading the go-ipfs config
	prevVal := uio.HAMTShardingSize
	uio.HAMTShardingSize = 1
	defer func() {
		uio.HAMTShardingSize = prevVal
	}()

	// Create and add a sharded directory
	dir := make(map[string]files.Node)
	// Make sure we have at least two levels of sharding
	for i := 0; i < uio.DefaultShardWidth+1; i++ {
		dir[strconv.Itoa(i)] = files.NewBytesFile([]byte(strconv.Itoa(i)))
	}

	r, err := a.Unixfs().Add(ctx, files.NewMapDirectory(dir), options.Unixfs.Pin(false))
	if err != nil {
		t.Fatal(err)
	}

	// Get the root of the directory
	nd, err := a.Dag().Get(ctx, r.RootCid())
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the root is a DagPB node (this API might change in the future to account for ADLs)
	_ = nd.(ipld.Node)
	pbNode := nd.(*merkledag.ProtoNode)

	// Remove one of the sharded directory blocks
	if err := a.Block().Rm(ctx, path.FromCid(pbNode.Links()[0].Cid)); err != nil {
		t.Fatal(err)
	}

	// Try and resolve each of the entries in the sharded directory which will result in pathing over the missing block
	//
	// Note: we could just check a particular path here, but it would require either greater use of the HAMT internals
	// or some hard coded values in the test both of which would be a pain to follow.
	for k := range dir {
		// The node will go out to the (non-existent) network looking for the missing block. Make sure we're erroring
		// because we exceeded the timeout on our query
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, time.Second*1)
		newPath, err := path.Join(r, k)
		if err != nil {
			t.Fatal(err)
		}

		_, err = a.ResolveNode(timeoutCtx, newPath)
		if err != nil {
			if timeoutCtx.Err() == nil {
				t.Fatal(err)
			}
		}
		timeoutCancel()
	}
}
