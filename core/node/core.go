package node

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	exchange "github.com/ipfs/boxo/exchange"
	offline "github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/boxo/fetcher"
	bsfetcher "github.com/ipfs/boxo/fetcher/impl/blockservice"
	"github.com/ipfs/boxo/filestore"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/mfs"
	pathresolver "github.com/ipfs/boxo/path/resolver"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/boxo/pinning/pinner/dspinner"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-unixfsnode"
	dagpb "github.com/ipld/go-codec-dagpb"
	"go.uber.org/fx"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/helpers"
	"github.com/ipfs/kubo/repo"
)

// BlockService creates new blockservice which provides an interface to fetch content-addressable blocks
func BlockService(cfg *config.Config) func(lc fx.Lifecycle, bs blockstore.Blockstore, rem exchange.Interface) blockservice.BlockService {
	return func(lc fx.Lifecycle, bs blockstore.Blockstore, rem exchange.Interface) blockservice.BlockService {
		bsvc := blockservice.New(bs, rem,
			blockservice.WriteThrough(cfg.Datastore.WriteThrough.WithDefault(config.DefaultWriteThrough)),
		)

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return bsvc.Close()
			},
		})

		return bsvc
	}
}

// Pinning creates new pinner which tells GC which blocks should be kept
func Pinning(bstore blockstore.Blockstore, ds format.DAGService, repo repo.Repo) (pin.Pinner, error) {
	rootDS := repo.Datastore()

	syncFn := func(ctx context.Context) error {
		if err := rootDS.Sync(ctx, blockstore.BlockPrefix); err != nil {
			return err
		}
		return rootDS.Sync(ctx, filestore.FilestorePrefix)
	}
	syncDs := &syncDagService{ds, syncFn}

	ctx := context.TODO()

	pinning, err := dspinner.New(ctx, rootDS, syncDs)
	if err != nil {
		return nil, err
	}

	return pinning, nil
}

var (
	_ merkledag.SessionMaker = new(syncDagService)
	_ format.DAGService      = new(syncDagService)
)

// syncDagService is used by the Pinner to ensure data gets persisted to the underlying datastore
type syncDagService struct {
	format.DAGService
	syncFn func(context.Context) error
}

func (s *syncDagService) Sync(ctx context.Context) error {
	return s.syncFn(ctx)
}

func (s *syncDagService) Session(ctx context.Context) format.NodeGetter {
	return merkledag.NewSession(ctx, s.DAGService)
}

// FetchersOut allows injection of fetchers.
type FetchersOut struct {
	fx.Out
	IPLDFetcher          fetcher.Factory `name:"ipldFetcher"`
	UnixfsFetcher        fetcher.Factory `name:"unixfsFetcher"`
	OfflineIPLDFetcher   fetcher.Factory `name:"offlineIpldFetcher"`
	OfflineUnixfsFetcher fetcher.Factory `name:"offlineUnixfsFetcher"`
}

// FetchersIn allows using fetchers for other dependencies.
type FetchersIn struct {
	fx.In
	IPLDFetcher          fetcher.Factory `name:"ipldFetcher"`
	UnixfsFetcher        fetcher.Factory `name:"unixfsFetcher"`
	OfflineIPLDFetcher   fetcher.Factory `name:"offlineIpldFetcher"`
	OfflineUnixfsFetcher fetcher.Factory `name:"offlineUnixfsFetcher"`
}

// FetcherConfig returns a fetcher config that can build new fetcher instances
func FetcherConfig(bs blockservice.BlockService) FetchersOut {
	ipldFetcher := bsfetcher.NewFetcherConfig(bs)
	ipldFetcher.PrototypeChooser = dagpb.AddSupportToChooser(bsfetcher.DefaultPrototypeChooser)
	unixFSFetcher := ipldFetcher.WithReifier(unixfsnode.Reify)

	// Construct offline versions which we can safely use in contexts where
	// path resolution should not fetch new blocks via exchange.
	offlineBs := blockservice.New(bs.Blockstore(), offline.Exchange(bs.Blockstore()))
	offlineIpldFetcher := bsfetcher.NewFetcherConfig(offlineBs)
	offlineIpldFetcher.PrototypeChooser = dagpb.AddSupportToChooser(bsfetcher.DefaultPrototypeChooser)
	offlineUnixFSFetcher := offlineIpldFetcher.WithReifier(unixfsnode.Reify)

	return FetchersOut{
		IPLDFetcher:          ipldFetcher,
		UnixfsFetcher:        unixFSFetcher,
		OfflineIPLDFetcher:   offlineIpldFetcher,
		OfflineUnixfsFetcher: offlineUnixFSFetcher,
	}
}

// PathResolversOut allows injection of path resolvers
type PathResolversOut struct {
	fx.Out
	IPLDPathResolver          pathresolver.Resolver `name:"ipldPathResolver"`
	UnixFSPathResolver        pathresolver.Resolver `name:"unixFSPathResolver"`
	OfflineIPLDPathResolver   pathresolver.Resolver `name:"offlineIpldPathResolver"`
	OfflineUnixFSPathResolver pathresolver.Resolver `name:"offlineUnixFSPathResolver"`
}

// PathResolverConfig creates path resolvers with the given fetchers.
func PathResolverConfig(fetchers FetchersIn) PathResolversOut {
	return PathResolversOut{
		IPLDPathResolver:          pathresolver.NewBasicResolver(fetchers.IPLDFetcher),
		UnixFSPathResolver:        pathresolver.NewBasicResolver(fetchers.UnixfsFetcher),
		OfflineIPLDPathResolver:   pathresolver.NewBasicResolver(fetchers.OfflineIPLDFetcher),
		OfflineUnixFSPathResolver: pathresolver.NewBasicResolver(fetchers.OfflineUnixfsFetcher),
	}
}

// Dag creates new DAGService
func Dag(bs blockservice.BlockService) format.DAGService {
	return merkledag.NewDAGService(bs)
}

// Files loads persisted MFS root
func Files(mctx helpers.MetricsCtx, lc fx.Lifecycle, repo repo.Repo, dag format.DAGService, bs blockstore.Blockstore) (*mfs.Root, error) {
	dsk := datastore.NewKey("/local/filesroot")
	pf := func(ctx context.Context, c cid.Cid) error {
		rootDS := repo.Datastore()
		if err := rootDS.Sync(ctx, blockstore.BlockPrefix); err != nil {
			return err
		}
		if err := rootDS.Sync(ctx, filestore.FilestorePrefix); err != nil {
			return err
		}

		if err := rootDS.Put(ctx, dsk, c.Bytes()); err != nil {
			return err
		}
		return rootDS.Sync(ctx, dsk)
	}

	var nd *merkledag.ProtoNode
	ctx := helpers.LifecycleCtx(mctx, lc)
	val, err := repo.Datastore().Get(ctx, dsk)

	switch {
	case err == datastore.ErrNotFound || val == nil:
		nd = unixfs.EmptyDirNode()
		err := dag.Add(ctx, nd)
		if err != nil {
			return nil, fmt.Errorf("failure writing filesroot to dagstore: %s", err)
		}
	case err == nil:
		c, err := cid.Cast(val)
		if err != nil {
			return nil, err
		}

		offineDag := merkledag.NewDAGService(blockservice.New(bs, offline.Exchange(bs)))
		rnd, err := offineDag.Get(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("error loading filesroot from dagservice: %s", err)
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
