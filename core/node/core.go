package node

import (
	"context"
	"fmt"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-mfs"
	"github.com/ipfs/go-unixfs"
	host "github.com/libp2p/go-libp2p-host"
	routing "github.com/libp2p/go-libp2p-routing"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/repo"
)

func BlockServiceCtor(lc fx.Lifecycle, bs blockstore.Blockstore, rem exchange.Interface) blockservice.BlockService {
	bsvc := blockservice.New(bs, rem)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return bsvc.Close()
		},
	})

	return bsvc
}

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

func DagCtor(bs blockservice.BlockService) format.DAGService {
	return merkledag.NewDAGService(bs)
}

func OnlineExchangeCtor(mctx MetricsCtx, lc fx.Lifecycle, host host.Host, rt routing.IpfsRouting, bs blockstore.GCBlockstore) exchange.Interface {
	bitswapNetwork := network.NewFromIpfsHost(host, rt)
	exch := bitswap.New(lifecycleCtx(mctx, lc), bitswapNetwork, bs)
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return exch.Close()
		},
	})
	return exch
}

func Files(mctx MetricsCtx, lc fx.Lifecycle, repo repo.Repo, dag format.DAGService) (*mfs.Root, error) {
	dsk := datastore.NewKey("/local/filesroot")
	pf := func(ctx context.Context, c cid.Cid) error {
		return repo.Datastore().Put(dsk, c.Bytes())
	}

	var nd *merkledag.ProtoNode
	val, err := repo.Datastore().Get(dsk)
	ctx := lifecycleCtx(mctx, lc)

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

type MetricsCtx context.Context
