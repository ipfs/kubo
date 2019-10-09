package fsnodes

import (
	"context"
	gopath "path"
	"time"

	"github.com/djdv/p9/p9"
	"github.com/djdv/p9/unimplfs"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

const ( //device - attempts to comply with standard multicodec table
	dMemory = 0x2f // generic "path"
	dIPFS   = 0xe4
)

var _ p9.File = (*Base)(nil)

type p9Path = uint64

// Base provides a foundation to build file system nodes which contain file meta data
// as well as stubs for unimplemented file system methods
type Base struct {
	// Provide stubs for unimplemented methods
	unimplfs.NoopFile
	p9.DefaultWalkGetAttr

	Trail []string // FS "breadcrumb" trail from node's root

	// Storage for file's metadata
	Qid      p9.QID
	meta     *p9.Attr
	metaMask *p9.AttrMask
	Logger   logging.EventLogger
}

func (b *Base) QID() p9.QID { return b.Qid }

func (b *Base) NinePath() p9Path { return b.Qid.Path }

func newBase(ops ...nodeopts.AttachOption) Base {
	options := nodeopts.AttachOps(ops...)

	return Base{
		Logger:   options.Logger,
		meta:     new(p9.Attr),
		metaMask: new(p9.AttrMask),
	}
}

func (b *Base) Derive() Base {
	newFid := newBase(nodeopts.Logger(b.Logger))

	newFid.Qid = b.Qid
	newFid.meta, newFid.metaMask = b.meta, b.metaMask
	newFid.Trail = make([]string, len(b.Trail))
	copy(newFid.Trail, b.Trail)

	return newFid
}

// IPFSBase is much like Base but extends it to hold IPFS specific metadata
type IPFSBase struct {
	Base
	OverlayFileMeta

	/* For file systems,
	this context should be set prior to `Attach`

	For files,
	this context should be overwritten with a context derived from the existing fs context
	during `Walk`

	The context is expected to be valid for the lifetime of the file system / file respectively
	to be used during operations, such as `Walk`, `Open`, `Read` etc.
	*/
	filesystemCtx context.Context
	// cancel should be called upon `Close`
	// closing a file system returned from `Attach`
	// or a derived file previously returned from `Walk`
	filesystemCancel context.CancelFunc

	// Typically you'll want to derive a context from the fs ctx within one operation (like Open)
	// use it with the CoreAPI for something (like Get)
	// and cancel it in another operation (like Close)
	// those pointers should be stored here between operation calls

	// Format the namespace as if it were a rooted directory, sans trailing slash
	// e.g. `/ipfs`
	// the base relative path is appended to the namespace for core requests upon calling `.CorePath()`
	coreNamespace string
	core          coreiface.CoreAPI
}

func (b *Base) StringPath() string {
	return gopath.Join(b.Trail...)
}

func (ib *IPFSBase) StringPath() string {
	return gopath.Join(append([]string{ib.coreNamespace}, ib.Base.StringPath())...)
}

func (ib *IPFSBase) CorePath(names ...string) corepath.Path {
	return corepath.Join(rootPath(ib.coreNamespace), append(ib.Trail, names...)...)
}

//func newIPFSBase(ctx context.Context, path corepath.Resolved, kind p9.FileMode, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) IPFSBase {
func newIPFSBase(ctx context.Context, coreNamespace string, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) IPFSBase {
	options := nodeopts.AttachOps(ops...)

	base := IPFSBase{
		Base:          newBase(ops...),
		coreNamespace: coreNamespace,
		core:          core,
	}
	base.filesystemCtx, base.filesystemCancel = context.WithCancel(ctx)

	if options.Parent != nil { // parent is optional
		parentRef, ok := options.Parent.(walkRef) // interface is not
		if !ok {
			panic("parent node lacks overlay traversal methods")
		}
		base.OverlayFileMeta.parent = parentRef
	}
	return base
}

func (ib *IPFSBase) Derive() IPFSBase {
	newFid := IPFSBase{
		Base:            ib.Base.Derive(),
		OverlayFileMeta: ib.OverlayFileMeta,
		coreNamespace:   ib.coreNamespace,
		core:            ib.core,
	}
	newFid.filesystemCtx, newFid.filesystemCancel = context.WithCancel(ib.filesystemCtx)

	return newFid
}

func (b *IPFSBase) Flush() error {
	b.Logger.Debugf("flushing: {%d}%q", b.Qid.Path, b.StringPath())
	return nil
}

func (b *Base) Close() error {
	b.Logger.Debugf("closing: {%d}%q", b.Qid.Path, b.StringPath())
	return nil
}

func (ib *IPFSBase) Close() error {
	ib.Logger.Debugf("closing: {%d}%q", ib.Qid.Path, ib.StringPath())

	var err error
	if ib.filesystemCancel != nil {
		if ib.proxy != nil {
			if err = ib.proxy.Close(); err != nil {
				ib.Logger.Error(err)
			}
		}
		ib.filesystemCancel()
	}

	return err
}

func (b *Base) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	b.Logger.Debugf("GetAttr")

	return b.Qid, *b.metaMask, *b.meta, nil
}

func (b *IPFSBase) callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(b.filesystemCtx, 30*time.Second)
}
