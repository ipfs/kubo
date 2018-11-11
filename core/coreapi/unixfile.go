package coreapi

import (
	"context"
	"errors"
	"io"
	"os"
	gopath "path"
	"time"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"

	ft "gx/ipfs/QmUnHNqhSB1JgzVCxL1Kz3yb4bdyB4q1Z9AD5AUBVmt3fZ/go-unixfs"
	uio "gx/ipfs/QmUnHNqhSB1JgzVCxL1Kz3yb4bdyB4q1Z9AD5AUBVmt3fZ/go-unixfs/io"
	files "gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"
	dag "gx/ipfs/QmcGt25mrjuB2kKW2zhPbXVZNHc4yoTDQ65NA8m6auP2f1/go-merkledag"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
)

// Number to file to prefetch in directories
// TODO: should we allow setting this via context hint?
const prefetchFiles = 4

// TODO: this probably belongs in go-unixfs (and could probably replace a chunk of it's interface in the long run)

type sizeInfo struct {
	size    int64
	name    string
	modTime time.Time
}

func (s *sizeInfo) Name() string {
	return s.name
}

func (s *sizeInfo) Size() int64 {
	return s.size
}

func (s *sizeInfo) Mode() os.FileMode {
	return 0444 // all read
}

func (s *sizeInfo) ModTime() time.Time {
	return s.modTime
}

func (s *sizeInfo) IsDir() bool {
	return false
}

func (s *sizeInfo) Sys() interface{} {
	return nil
}

type ufsDirectory struct {
	ctx   context.Context
	dserv ipld.DAGService

	files chan *ipld.Link

	name string
	path string
}

func (d *ufsDirectory) Close() error {
	return files.ErrNotReader
}

func (d *ufsDirectory) Read(_ []byte) (int, error) {
	return 0, files.ErrNotReader
}

func (d *ufsDirectory) FileName() string {
	return d.name
}

func (d *ufsDirectory) FullPath() string {
	return d.path
}

func (d *ufsDirectory) IsDirectory() bool {
	return true
}

func (d *ufsDirectory) NextFile() (files.File, error) {
	l, ok := <-d.files
	if !ok {
		return nil, io.EOF
	}

	nd, err := l.GetNode(d.ctx, d.dserv)
	if err != nil {
		return nil, err
	}

	return newUnixfsFile(d.ctx, d.dserv, nd, l.Name, d)
}

func (d *ufsDirectory) Size() (int64, error) {
	return 0, files.ErrNotReader
}

func (d *ufsDirectory) Seek(offset int64, whence int) (int64, error) {
	return 0, files.ErrNotReader
}

type ufsFile struct {
	uio.DagReader

	name string
	path string
}

func (f *ufsFile) IsDirectory() bool {
	return false
}

func (f *ufsFile) NextFile() (files.File, error) {
	return nil, files.ErrNotDirectory
}

func (f *ufsFile) FileName() string {
	return f.name
}

func (f *ufsFile) FullPath() string {
	return f.path
}

func (f *ufsFile) Size() (int64, error) {
	return int64(f.DagReader.Size()), nil
}

func newUnixfsDir(ctx context.Context, dserv ipld.DAGService, nd ipld.Node, name string, path string) (iface.UnixfsFile, error) {
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

		name: name,
		path: path,
	}, nil
}

func newUnixfsFile(ctx context.Context, dserv ipld.DAGService, nd ipld.Node, name string, parent files.File) (iface.UnixfsFile, error) {
	path := name
	if parent != nil {
		path = gopath.Join(parent.FullPath(), name)
	}

	switch dn := nd.(type) {
	case *dag.ProtoNode:
		fsn, err := ft.FSNodeFromBytes(dn.Data())
		if err != nil {
			return nil, err
		}
		if fsn.IsDir() {
			return newUnixfsDir(ctx, dserv, nd, name, path)
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

		name: name,
		path: path,
	}, nil
}

var _ os.FileInfo = &sizeInfo{}
