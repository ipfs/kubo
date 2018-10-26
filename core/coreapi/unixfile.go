package coreapi

import (
	"context"
	"errors"
	"io"

	files "gx/ipfs/QmXWZCd8jfaHmt4UDSnjKmGcrQMw95bDGWqEeVLVJjoANX/go-ipfs-files"
	ft "gx/ipfs/Qmbvw7kpSM2p6rbQ57WGRhhqNfCiNGW6EKH4xgHLw4bsnB/go-unixfs"
	uio "gx/ipfs/Qmbvw7kpSM2p6rbQ57WGRhhqNfCiNGW6EKH4xgHLw4bsnB/go-unixfs/io"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
	dag "gx/ipfs/QmdV35UHnL1FM52baPkeUo6u7Fxm2CRUkPTLRPxeF8a4Ap/go-merkledag"
)

// Number to file to prefetch in directories
// TODO: should we allow setting this via context hint?
const prefetchFiles = 4

// TODO: this probably belongs in go-unixfs (and could probably replace a chunk of it's interface in the long run)

type ufsDirectory struct {
	ctx   context.Context
	dserv ipld.DAGService

	files chan *ipld.Link
}

func (d *ufsDirectory) Close() error {
	return files.ErrNotReader
}

func (d *ufsDirectory) Read(_ []byte) (int, error) {
	return 0, files.ErrNotReader
}

func (d *ufsDirectory) IsDirectory() bool {
	return true
}

func (d *ufsDirectory) NextFile() (string, files.File, error) {
	l, ok := <-d.files
	if !ok {
		return "", nil, io.EOF
	}

	nd, err := l.GetNode(d.ctx, d.dserv)
	if err != nil {
		return "", nil, err
	}

	f, err := newUnixfsFile(d.ctx, d.dserv, nd, d)
	return l.Name, f, err
}

func (d *ufsDirectory) Size() (int64, error) {
	return 0, files.ErrNotReader
}

func (d *ufsDirectory) Seek(offset int64, whence int) (int64, error) {
	return 0, files.ErrNotReader
}

type ufsFile struct {
	uio.DagReader
}

func (f *ufsFile) IsDirectory() bool {
	return false
}

func (f *ufsFile) NextFile() (string, files.File, error) {
	return "", nil, files.ErrNotDirectory
}

func (f *ufsFile) Size() (int64, error) {
	return int64(f.DagReader.Size()), nil
}

func newUnixfsDir(ctx context.Context, dserv ipld.DAGService, nd ipld.Node) (files.File, error) {
	dir, err := uio.NewDirectoryFromNode(dserv, nd)
	if err != nil {
		return nil, err
	}

	fileCh := make(chan *ipld.Link, prefetchFiles)
	go func() {
		dir.ForEachLink(ctx, func(link *ipld.Link) error {
			select {
			case fileCh <- link:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})

		close(fileCh)
	}()

	return &ufsDirectory{
		ctx:   ctx,
		dserv: dserv,

		files: fileCh,
	}, nil
}

func newUnixfsFile(ctx context.Context, dserv ipld.DAGService, nd ipld.Node, parent files.File) (files.File, error) {
	switch dn := nd.(type) {
	case *dag.ProtoNode:
		fsn, err := ft.FSNodeFromBytes(dn.Data())
		if err != nil {
			return nil, err
		}
		if fsn.IsDir() {
			return newUnixfsDir(ctx, dserv, nd)
		}

	case *dag.RawNode:
	default:
		return nil, errors.New("unknown node type")
	}

	dr, err := uio.NewDagReader(ctx, nd, dserv)
	if err != nil {
		return nil, err
	}

	return &ufsFile{
		DagReader: dr,
	}, nil
}
