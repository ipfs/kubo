package coreapi

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-ipfs/core"

	"github.com/ipfs/go-ipfs/core/coreunix"

	blockservice "github.com/ipfs/go-blockservice"
	cid "github.com/ipfs/go-cid"
	cidutil "github.com/ipfs/go-cidutil"
	filestore "github.com/ipfs/go-filestore"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	merkledag "github.com/ipfs/go-merkledag"
	dagtest "github.com/ipfs/go-merkledag/test"
	mfs "github.com/ipfs/go-mfs"
	ft "github.com/ipfs/go-unixfs"
	unixfile "github.com/ipfs/go-unixfs/file"
	uio "github.com/ipfs/go-unixfs/io"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	path "github.com/ipfs/interface-go-ipfs-core/path"
)

type UnixfsAPI CoreAPI

var nilNode *core.IpfsNode
var once sync.Once

func getOrCreateNilNode() (*core.IpfsNode, error) {
	once.Do(func() {
		if nilNode != nil {
			return
		}
		node, err := core.NewNode(context.Background(), &core.BuildCfg{
			//TODO: need this to be true or all files
			// hashed will be stored in memory!
			NilRepo: true,
		})
		if err != nil {
			panic(err)
		}
		nilNode = node
	})

	return nilNode, nil
}

// Add builds a merkledag node from a reader, adds it to the blockstore,
// and returns the key representing that node.
func (api *UnixfsAPI) Add(ctx context.Context, files files.Node, opts ...options.UnixfsAddOption) (path.Resolved, error) {
	settings, prefix, err := options.UnixfsAddOptions(opts...)
	if err != nil {
		return nil, err
	}

	cfg, err := api.repo.Config()
	if err != nil {
		return nil, err
	}

	// check if repo will exceed storage limit if added
	// TODO: this doesn't handle the case if the hashed file is already in blocks (deduplicated)
	// TODO: conditional GC is disabled due to it is somehow not possible to pass the size to the daemon
	//if err := corerepo.ConditionalGC(req.Context(), n, uint64(size)); err != nil {
	//	res.SetError(err, cmds.ErrNormal)
	//	return
	//}

	if settings.NoCopy && !(cfg.Experimental.FilestoreEnabled || cfg.Experimental.UrlstoreEnabled) {
		return nil, fmt.Errorf("either the filestore or the urlstore must be enabled to use nocopy, see: https://git.io/vNItf")
	}

	addblockstore := api.blockstore
	if !(settings.FsCache || settings.NoCopy) {
		addblockstore = bstore.NewGCBlockstore(api.baseBlocks, api.blockstore)
	}
	exch := api.exchange
	pinning := api.pinning

	if settings.OnlyHash {
		node, err := getOrCreateNilNode()
		if err != nil {
			return nil, err
		}
		addblockstore = node.Blockstore
		exch = node.Exchange
		pinning = node.Pinning
	}

	bserv := blockservice.New(addblockstore, exch) // hash security 001
	dserv := dag.NewDAGService(bserv)

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
				ds := api.repo.Datastore()
				if err := ds.Sync(ctx, bstore.BlockPrefix); err != nil {
					return err
				}
				return ds.Sync(ctx, filestore.FilestorePrefix)
			},
		}
	}

	fileAdder, err := coreunix.NewAdder(ctx, pinning, addblockstore, syncDserv)
	if err != nil {
		return nil, err
	}

	fileAdder.Chunker = settings.Chunker
	if settings.Events != nil {
		fileAdder.Out = settings.Events
		fileAdder.Progress = settings.Progress
	}
	fileAdder.Pin = settings.Pin && !settings.OnlyHash
	fileAdder.Silent = settings.Silent
	fileAdder.RawLeaves = settings.RawLeaves
	fileAdder.NoCopy = settings.NoCopy
	fileAdder.CidBuilder = prefix

	switch settings.Layout {
	case options.BalancedLayout:
		// Default
	case options.TrickleLayout:
		fileAdder.Trickle = true
	default:
		return nil, fmt.Errorf("unknown layout: %d", settings.Layout)
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
		emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
		mr, err := mfs.NewRoot(ctx, md, emptyDirNode, nil)
		if err != nil {
			return nil, err
		}

		fileAdder.SetMfsRoot(mr)
	}

	nd, err := fileAdder.AddAllAndPin(ctx, files)
	if err != nil {
		return nil, err
	}

	if !settings.OnlyHash {
		if err := api.provider.Provide(nd.Cid()); err != nil {
			return nil, err
		}
	}

	return path.IpfsPath(nd.Cid()), nil
}

func (api *UnixfsAPI) Get(ctx context.Context, p path.Path) (files.Node, error) {
	ses := api.core().getSession(ctx)

	nd, err := ses.ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	return unixfile.NewUnixfsFile(ctx, ses.dag, nd)
}

// Ls returns the contents of an IPFS or IPNS object(s) at path p, with the format:
// `<link base58 hash> <link size in bytes> <link name>`
func (api *UnixfsAPI) Ls(ctx context.Context, p path.Path, opts ...options.UnixfsLsOption) (<-chan coreiface.DirEntry, error) {
	settings, err := options.UnixfsLsOptions(opts...)
	if err != nil {
		return nil, err
	}

	ses := api.core().getSession(ctx)
	uses := (*UnixfsAPI)(ses)

	dagnode, err := ses.ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	dir, err := uio.NewDirectoryFromNode(ses.dag, dagnode)
	if err == uio.ErrNotADir {
		return uses.lsFromLinks(ctx, dagnode.Links(), settings)
	}
	if err != nil {
		return nil, err
	}

	return uses.lsFromLinksAsync(ctx, dir, settings)
}

func (api *UnixfsAPI) processLink(ctx context.Context, linkres ft.LinkResult, settings *options.UnixfsLsSettings) coreiface.DirEntry {
	if linkres.Err != nil {
		return coreiface.DirEntry{Err: linkres.Err}
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
		if !settings.ResolveChildren {
			break
		}

		linkNode, err := linkres.Link.GetNode(ctx, api.dag)
		if err != nil {
			lnk.Err = err
			break
		}

		if pn, ok := linkNode.(*merkledag.ProtoNode); ok {
			d, err := ft.FSNodeFromBytes(pn.Data())
			if err != nil {
				lnk.Err = err
				break
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
			lnk.Size = d.FileSize()
		}
	}

	return lnk
}

func (api *UnixfsAPI) lsFromLinksAsync(ctx context.Context, dir uio.Directory, settings *options.UnixfsLsSettings) (<-chan coreiface.DirEntry, error) {
	out := make(chan coreiface.DirEntry)

	go func() {
		defer close(out)
		for l := range dir.EnumLinksAsync(ctx) {
			select {
			case out <- api.processLink(ctx, l, settings): //TODO: perf: processing can be done in background and in parallel
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

func (api *UnixfsAPI) lsFromLinks(ctx context.Context, ndlinks []*ipld.Link, settings *options.UnixfsLsSettings) (<-chan coreiface.DirEntry, error) {
	links := make(chan coreiface.DirEntry, len(ndlinks))
	for _, l := range ndlinks {
		lr := ft.LinkResult{Link: &ipld.Link{Name: l.Name, Size: l.Size, Cid: l.Cid}}

		links <- api.processLink(ctx, lr, settings) //TODO: can be parallel if settings.Async
	}
	close(links)
	return links, nil
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
