package coreapi

import (
	"context"
	"errors"
	"fmt"

	blockservice "github.com/ipfs/boxo/blockservice"
	bstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/files"
	filestore "github.com/ipfs/boxo/filestore"
	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	dagtest "github.com/ipfs/boxo/ipld/merkledag/test"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	unixfile "github.com/ipfs/boxo/ipld/unixfs/file"
	uio "github.com/ipfs/boxo/ipld/unixfs/io"
	"github.com/ipfs/boxo/mfs"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/boxo/provider"
	cid "github.com/ipfs/go-cid"
	cidutil "github.com/ipfs/go-cidutil"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/core/coreunix"
	"github.com/ipfs/kubo/tracing"
	mh "github.com/multiformats/go-multihash"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var log = logging.Logger("coreapi")

type UnixfsAPI CoreAPI

// Add builds a merkledag node from a reader, adds it to the blockstore,
// and returns the key representing that node.
func (api *UnixfsAPI) Add(ctx context.Context, files files.Node, opts ...options.UnixfsAddOption) (path.ImmutablePath, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.UnixfsAPI", "Add")
	defer span.End()

	settings, prefix, err := options.UnixfsAddOptions(opts...)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	span.SetAttributes(
		attribute.String("chunker", settings.Chunker),
		attribute.Int("cidversion", settings.CidVersion),
		attribute.Bool("inline", settings.Inline),
		attribute.Int("inlinelimit", settings.InlineLimit),
		attribute.Bool("rawleaves", settings.RawLeaves),
		attribute.Bool("rawleavesset", settings.RawLeavesSet),
		attribute.Int("maxfilelinks", settings.MaxFileLinks),
		attribute.Bool("maxfilelinksset", settings.MaxFileLinksSet),
		attribute.Int("maxdirectorylinks", settings.MaxDirectoryLinks),
		attribute.Bool("maxdirectorylinksset", settings.MaxDirectoryLinksSet),
		attribute.Int("maxhamtfanout", settings.MaxHAMTFanout),
		attribute.Bool("maxhamtfanoutset", settings.MaxHAMTFanoutSet),
		attribute.Int("layout", int(settings.Layout)),
		attribute.Bool("pin", settings.Pin),
		attribute.String("pin-name", settings.PinName),
		attribute.Bool("onlyhash", settings.OnlyHash),
		attribute.Bool("fscache", settings.FsCache),
		attribute.Bool("nocopy", settings.NoCopy),
		attribute.Bool("silent", settings.Silent),
		attribute.Bool("progress", settings.Progress),
	)

	cfg, err := api.repo.Config()
	if err != nil {
		return path.ImmutablePath{}, err
	}

	// check if repo will exceed storage limit if added
	// TODO: this doesn't handle the case if the hashed file is already in blocks (deduplicated)
	// TODO: conditional GC is disabled due to it is somehow not possible to pass the size to the daemon
	//if err := corerepo.ConditionalGC(req.Context(), n, uint64(size)); err != nil {
	//	res.SetError(err, cmds.ErrNormal)
	//	return
	//}

	if settings.NoCopy && !(cfg.Experimental.FilestoreEnabled || cfg.Experimental.UrlstoreEnabled) {
		return path.ImmutablePath{}, errors.New("either the filestore or the urlstore must be enabled to use nocopy, see: https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-filestore")
	}

	addblockstore := api.blockstore
	if !(settings.FsCache || settings.NoCopy) {
		addblockstore = bstore.NewGCBlockstore(api.baseBlocks, api.blockstore)
	}
	exch := api.exchange
	pinning := api.pinning

	if settings.OnlyHash {
		// setup a /dev/null pipeline to simulate adding the data
		dstore := dssync.MutexWrap(ds.NewNullDatastore())
		bs := bstore.NewBlockstore(dstore, bstore.WriteThrough(true)) // we use NewNullDatastore, so ok to always WriteThrough when OnlyHash
		addblockstore = bstore.NewGCBlockstore(bs, nil)               // gclocker will never be used
		exch = nil                                                    // exchange will never be used
		pinning = nil                                                 // pinner will never be used
	}

	bserv := blockservice.New(addblockstore, exch,
		blockservice.WriteThrough(cfg.Datastore.WriteThrough.WithDefault(config.DefaultWriteThrough)),
	) // hash security 001

	var dserv ipld.DAGService = merkledag.NewDAGService(bserv)

	// wrap the DAGService in a providingDAG service which provides every block written.
	// note about strategies:
	//   - "all" gets handled directly at the blockstore so no need to provide
	//   - "roots" gets handled in the pinner
	//   - "mfs" gets handled in mfs
	// We need to provide the "pinned" cases only. Added blocks are not
	// going to be provided by the blockstore (wrong strategy for that),
	// nor by the pinner (the pinner doesn't traverse the pinned DAG itself, it only
	// handles roots). This wrapping ensures all blocks of pinned content get provided.
	if settings.Pin && !settings.OnlyHash &&
		(api.providingStrategy&config.ProvideStrategyPinned) != 0 {
		dserv = &providingDagService{dserv, api.provider}
	}

	// add a sync call to the DagService
	// this ensures that data written to the DagService is persisted to the underlying datastore
	// TODO: propagate the Sync function from the datastore through the blockstore, blockservice and dagservice
	var syncDserv *syncDagService
	if settings.OnlyHash {
		syncDserv = &syncDagService{
			DAGService: dserv,
			syncFn:     func() error { return nil },
		}
	} else {
		syncDserv = &syncDagService{
			DAGService: dserv,
			syncFn: func() error {
				rds := api.repo.Datastore()
				if err := rds.Sync(ctx, bstore.BlockPrefix); err != nil {
					return err
				}
				return rds.Sync(ctx, filestore.FilestorePrefix)
			},
		}
	}

	// Note: the dag service gets wrapped multiple times:
	// 1. providingDagService (if pinned strategy) - provides blocks as they're added
	// 2. syncDagService - ensures data persistence
	// 3. batchingDagService (in coreunix.Adder) - batches operations for efficiency

	fileAdder, err := coreunix.NewAdder(ctx, pinning, addblockstore, syncDserv)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	fileAdder.Chunker = settings.Chunker
	if settings.Events != nil {
		fileAdder.Out = settings.Events
		fileAdder.Progress = settings.Progress
	}
	fileAdder.Pin = settings.Pin && !settings.OnlyHash
	if settings.Pin {
		fileAdder.PinName = settings.PinName
	}
	fileAdder.Silent = settings.Silent
	fileAdder.RawLeaves = settings.RawLeaves
	if settings.MaxFileLinksSet {
		fileAdder.MaxLinks = settings.MaxFileLinks
	}
	if settings.MaxDirectoryLinksSet {
		fileAdder.MaxDirectoryLinks = settings.MaxDirectoryLinks
	}

	if settings.MaxHAMTFanoutSet {
		fileAdder.MaxHAMTFanout = settings.MaxHAMTFanout
	}
	fileAdder.NoCopy = settings.NoCopy
	fileAdder.CidBuilder = prefix
	fileAdder.PreserveMode = settings.PreserveMode
	fileAdder.PreserveMtime = settings.PreserveMtime
	fileAdder.FileMode = settings.Mode
	fileAdder.FileMtime = settings.Mtime

	switch settings.Layout {
	case options.BalancedLayout:
		// Default
	case options.TrickleLayout:
		fileAdder.Trickle = true
	default:
		return path.ImmutablePath{}, fmt.Errorf("unknown layout: %d", settings.Layout)
	}

	if settings.Inline {
		fileAdder.CidBuilder = cidutil.InlineBuilder{
			Builder: fileAdder.CidBuilder,
			Limit:   settings.InlineLimit,
		}
	}

	if settings.OnlyHash {
		md := dagtest.Mock()
		emptyDirNode := ft.EmptyDirNode()
		// Use the same prefix for the "empty" MFS root as for the file adder.
		err := emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
		if err != nil {
			return path.ImmutablePath{}, err
		}
		// MFS root for OnlyHash mode: provider is nil since we're not storing/providing anything
		mr, err := mfs.NewRoot(ctx, md, emptyDirNode, nil, nil)
		if err != nil {
			return path.ImmutablePath{}, err
		}

		fileAdder.SetMfsRoot(mr)
	}

	nd, err := fileAdder.AddAllAndPin(ctx, files)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	return path.FromCid(nd.Cid()), nil
}

func (api *UnixfsAPI) Get(ctx context.Context, p path.Path) (files.Node, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.UnixfsAPI", "Get", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	ses := api.core().getSession(ctx)

	nd, err := ses.ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	return unixfile.NewUnixfsFile(ctx, ses.dag, nd)
}

// Ls returns the contents of an IPFS or IPNS object(s) at path p, with the format:
// `<link base58 hash> <link size in bytes> <link name>`
func (api *UnixfsAPI) Ls(ctx context.Context, p path.Path, out chan<- coreiface.DirEntry, opts ...options.UnixfsLsOption) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.UnixfsAPI", "Ls", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	defer close(out)

	settings, err := options.UnixfsLsOptions(opts...)
	if err != nil {
		return err
	}

	span.SetAttributes(attribute.Bool("resolvechildren", settings.ResolveChildren))

	ses := api.core().getSession(ctx)
	uses := (*UnixfsAPI)(ses)

	dagnode, err := ses.ResolveNode(ctx, p)
	if err != nil {
		return err
	}

	dir, err := uio.NewDirectoryFromNode(ses.dag, dagnode)
	if err != nil {
		if errors.Is(err, uio.ErrNotADir) {
			return uses.lsFromLinks(ctx, dagnode.Links(), settings, out)
		}
		return err
	}

	return uses.lsFromDirLinks(ctx, dir, settings, out)
}

func (api *UnixfsAPI) processLink(ctx context.Context, linkres ft.LinkResult, settings *options.UnixfsLsSettings) (coreiface.DirEntry, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.UnixfsAPI", "ProcessLink")
	defer span.End()
	if linkres.Link != nil {
		span.SetAttributes(attribute.String("linkname", linkres.Link.Name), attribute.String("cid", linkres.Link.Cid.String()))
	}

	if linkres.Err != nil {
		return coreiface.DirEntry{}, linkres.Err
	}

	lnk := coreiface.DirEntry{
		Name: linkres.Link.Name,
		Cid:  linkres.Link.Cid,
	}

	switch lnk.Cid.Type() {
	case cid.Raw:
		// No need to check with raw leaves
		lnk.Type = coreiface.TFile
		lnk.Size = linkres.Link.Size
	case cid.DagProtobuf:
		if settings.ResolveChildren {
			linkNode, err := linkres.Link.GetNode(ctx, api.dag)
			if err != nil {
				return coreiface.DirEntry{}, err
			}

			if pn, ok := linkNode.(*merkledag.ProtoNode); ok {
				d, err := ft.FSNodeFromBytes(pn.Data())
				if err != nil {
					return coreiface.DirEntry{}, err
				}
				switch d.Type() {
				case ft.TFile, ft.TRaw:
					lnk.Type = coreiface.TFile
				case ft.THAMTShard, ft.TDirectory, ft.TMetadata:
					lnk.Type = coreiface.TDirectory
				case ft.TSymlink:
					lnk.Type = coreiface.TSymlink
					lnk.Target = string(d.Data())
				}
				if !settings.UseCumulativeSize {
					lnk.Size = d.FileSize()
				}
				lnk.Mode = d.Mode()
				lnk.ModTime = d.ModTime()
			}
		}

		if settings.UseCumulativeSize {
			lnk.Size = linkres.Link.Size
		}
	}

	return lnk, nil
}

func (api *UnixfsAPI) lsFromDirLinks(ctx context.Context, dir uio.Directory, settings *options.UnixfsLsSettings, out chan<- coreiface.DirEntry) error {
	for l := range dir.EnumLinksAsync(ctx) {
		dirEnt, err := api.processLink(ctx, l, settings) // TODO: perf: processing can be done in background and in parallel
		if err != nil {
			return err
		}
		select {
		case out <- dirEnt:
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

func (api *UnixfsAPI) lsFromLinks(ctx context.Context, ndlinks []*ipld.Link, settings *options.UnixfsLsSettings, out chan<- coreiface.DirEntry) error {
	// Create links channel large enough to not block when writing to out is slower.
	links := make(chan coreiface.DirEntry, len(ndlinks))
	errs := make(chan error, 1)
	go func() {
		defer close(links)
		defer close(errs)
		for _, l := range ndlinks {
			lr := ft.LinkResult{Link: &ipld.Link{Name: l.Name, Size: l.Size, Cid: l.Cid}}
			lnk, err := api.processLink(ctx, lr, settings) // TODO: can be parallel if settings.Async
			if err != nil {
				errs <- err
				return
			}
			select {
			case links <- lnk:
			case <-ctx.Done():
				return
			}
		}
	}()

	for lnk := range links {
		out <- lnk
	}
	return <-errs
}

func (api *UnixfsAPI) core() *CoreAPI {
	return (*CoreAPI)(api)
}

// syncDagService is used by the Adder to ensure blocks get persisted to the underlying datastore
type syncDagService struct {
	ipld.DAGService
	syncFn func() error
}

func (s *syncDagService) Sync() error {
	return s.syncFn()
}

type providingDagService struct {
	ipld.DAGService
	provider.MultihashProvider
}

func (pds *providingDagService) Add(ctx context.Context, n ipld.Node) error {
	if err := pds.DAGService.Add(ctx, n); err != nil {
		return err
	}
	// Provider errors are logged but not propagated.
	// We don't want DAG operations to fail due to providing issues.
	// The user's data is still stored successfully even if the
	// announcement to the routing system fails temporarily.
	if err := pds.StartProviding(false, n.Cid().Hash()); err != nil {
		log.Errorf("failed to provide new block: %s", err)
	}
	return nil
}

func (pds *providingDagService) AddMany(ctx context.Context, nds []ipld.Node) error {
	if err := pds.DAGService.AddMany(ctx, nds); err != nil {
		return err
	}
	keys := make([]mh.Multihash, len(nds))
	for i, n := range nds {
		keys[i] = n.Cid().Hash()
	}
	// Same error handling philosophy as Add(): log but don't fail.
	if err := pds.StartProviding(false, keys...); err != nil {
		log.Errorf("failed to provide new blocks: %s", err)
	}
	return nil
}

var _ ipld.DAGService = (*providingDagService)(nil)
