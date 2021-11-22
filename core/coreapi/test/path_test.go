package test

import (
	"context"
	"strconv"
	"testing"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-merkledag"
	uio "github.com/ipfs/go-unixfs/io"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipld/go-ipld-prime"
)

func TestPathUnixFSHAMTPartial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a node
	apis, err := NodeProvider{}.MakeAPISwarm(ctx, true, 1)
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
	nd, err := a.Dag().Get(ctx, r.Cid())
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the root is a DagPB node (this API might change in the future to account for ADLs)
	_ = nd.(ipld.Node)
	pbNode := nd.(*merkledag.ProtoNode)

	// Remove one of the sharded directory blocks
	if err := a.Block().Rm(ctx, path.IpfsPath(pbNode.Links()[0].Cid)); err != nil {
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
		_, err := a.ResolveNode(timeoutCtx, path.Join(r, k))
		if err != nil {
			if timeoutCtx.Err() == nil {
				t.Fatal(err)
			}
		}
		timeoutCancel()
	}
}
