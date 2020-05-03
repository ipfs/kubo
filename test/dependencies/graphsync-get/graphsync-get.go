package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-graphsync"
	gsimpl "github.com/ipfs/go-graphsync/impl"
	"github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	"github.com/ipfs/go-merkledag"
	uio "github.com/ipfs/go-unixfs/io"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	ipldselector "github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

func newGraphsync(ctx context.Context, p2p host.Host, bs blockstore.Blockstore) (graphsync.GraphExchange, error) {
	network := network.NewFromLibp2pHost(p2p)
	return gsimpl.New(ctx,
		network,
		storeutil.LoaderForBlockstore(bs),
		storeutil.StorerForBlockstore(bs),
	), nil
}

var selectAll ipld.Node = func() ipld.Node {
	ssb := builder.NewSelectorSpecBuilder(basicnode.Style.Any)
	return ssb.ExploreRecursive(
		ipldselector.RecursionLimitDepth(100), // default max
		ssb.ExploreAll(ssb.ExploreRecursiveEdge()),
	).Node()
}()

func fetch(ctx context.Context, gs graphsync.GraphExchange, p peer.ID, c cid.Cid) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resps, errs := gs.Request(ctx, p, cidlink.Link{Cid: c}, selectAll)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-resps:
			if !ok {
				resps = nil
			}
		case err, ok := <-errs:
			if !ok {
				// done.
				return nil
			}
			if err != nil {
				return fmt.Errorf("got an unexpected error: %s", err)
			}
		}
	}
}

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("expected a multiaddr and a CID, got %d args", len(os.Args)-1)
	}
	addr, err := multiaddr.NewMultiaddr(os.Args[1])
	if err != nil {
		log.Fatalf("failed to multiaddr '%q': %s", os.Args[1], err)
	}
	ai, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		log.Fatal(err)
	}

	target, err := cid.Decode(os.Args[2])
	if err != nil {
		log.Fatalf("failed to decode CID '%q': %s", os.Args[2], err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p2p, err := libp2p.New(ctx, libp2p.NoListenAddrs)
	if err != nil {
		log.Fatal(err)
	}
	err = p2p.Connect(ctx, *ai)
	if err != nil {
		log.Fatal(err)
	}

	bs := blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	gs, err := newGraphsync(ctx, p2p, bs)
	if err != nil {
		log.Fatal("failed to start", err)
	}
	err = fetch(ctx, gs, ai.ID, target)
	if err != nil {
		log.Fatal(err)
	}

	dag := merkledag.NewDAGService(blockservice.New(bs, offline.Exchange(bs)))
	root, err := dag.Get(ctx, target)
	if err != nil {
		log.Fatal(err)
	}
	reader, err := uio.NewDagReader(ctx, root, dag)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		log.Fatal(err)
	}
}
