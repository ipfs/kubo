package node

import (
	"context"
	"fmt"

	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/repo"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/ipfs/go-ipfs-exchange-offline"
	"github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-mfs"
	"github.com/ipfs/go-unixfs"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	"go.uber.org/fx"
)

// BlockService creates new blockservice which provides an interface to fetch content-addressable blocks
func BlockService(lc fx.Lifecycle, bs blockstore.Blockstore, rem exchange.Interface) blockservice.BlockService {
	bsvc := blockservice.New(bs, rem)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return bsvc.Close()
		},
	})

	return bsvc
}

// Pinning creates new pinner which tells GC which blocks should be kept
func Pinning(bstore blockstore.Blockstore, ds format.DAGService, repo repo.Repo) (pin.Pinner, error) {
	internalDag := merkledag.NewDAGService(blockservice.New(bstore, offline.Exchange(bstore)))
	pinning, err := pin.LoadPinner(repo.Datastore(), ds, internalDag)
	if err != nil {
		// TODO: we should move towards only running 'NewPinner' explicitly on
		// node init instead of implicitly here as a result of the pinner keys
		// not being found in the datastore.
		// this is kinda sketchy and could cause data loss
		pinning = pin.NewPinner(repo.Datastore(), ds, internalDag)
	}

	return pinning, nil
}

// Dag creates new DAGService
func Dag(bs blockservice.BlockService) format.DAGService {
	return merkledag.NewDAGService(bs)
}

// OnlineExchange creates new LibP2P backed block exchange (BitSwap)
func OnlineExchange(provide bool) interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, host host.Host, rt routing.Routing, bs blockstore.GCBlockstore) exchange.Interface {
		bitswapNetwork := network.NewFromIpfsHost(host, rt)
		exch := bitswap.New(helpers.LifecycleCtx(mctx, lc), bitswapNetwork, bs, bitswap.ProvideEnabled(provide))
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return exch.Close()
			},
		})
		return exch

	}
}

// Files loads persisted MFS root
func Files(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo, dag format.DAGService) (*mfs.Root, error) {
	dsk := datastore.NewKey("/local/filesroot")
	pf := func(ctx context.Context, c cid.Cid) error {
		return repo.Datastore().Put(dsk, c.Bytes())
	}

	var nd *merkledag.ProtoNode
	val, err := repo.Datastore().Get(dsk)
	ctx := helpers.LifecycleCtx(mctx, lc)

	switch {
	case err == datastore.ErrNotFound || val == nil:
		nd = unixfs.EmptyDirNode()
		err := dag.Add(ctx, nd)
		if err != nil {
			return nil, fmt.Errorf("failure writing to dagstore: %s", err)
		}
	case err == nil:
		c, err := cid.Cast(val)
		if err != nil {
			return nil, err
		}

		rnd, err := dag.Get(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("error loading filesroot from DAG: %s", err)
		}

		pbnd, ok := rnd.(*merkledag.ProtoNode)
		if !ok {
			return nil, merkledag.ErrNotProtobuf
		}

		nd = pbnd
	default:
		return nil, err
	}

	root, err := mfs.NewRoot(ctx, dag, nd, pf)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return root.Close()
		},
	})

	return root, err
}
