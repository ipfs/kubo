package fsnodes

import (
	"context"
	"errors"
	gopath "path"
	"time"

	"github.com/hugelgupf/p9/p9"
	"github.com/hugelgupf/p9/unimplfs"
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
	Qid      *p9.QID
	meta     *p9.Attr
	metaMask *p9.AttrMask
	Logger   logging.EventLogger

	closed   bool // set to true upon close; this reference should not be used again for anything
	modified bool // set to true when the `Trail` has been modified (usually by `Step`)
	// reset to false when `Qid` has been populated with the current path in `Trail` (usually by `QID`)

}

func newBase(ops ...nodeopts.AttachOption) Base {
	options := nodeopts.AttachOps(ops...)

	return Base{
		Logger:   options.Logger,
		Qid:      new(p9.QID),
		meta:     new(p9.Attr),
		metaMask: new(p9.AttrMask),
	}
}

func (b *Base) clone() Base {
	return *b
}

func (b *Base) Fork() Base {
	newFid := b.clone()
	newFid.Trail = make([]string, len(b.Trail)+1) // NOTE: +1 is preallocated space for `Step`; not required
	copy(newFid.Trail, b.Trail)

	return newFid
}

func (b *Base) String() string {
	return gopath.Join(b.Trail...)
}

func (b *Base) NinePath() p9Path { return b.Qid.Path }

func (b *Base) QID() (p9.QID, error) { return *b.Qid, nil }

// IPFSBase is much like Base but extends it to hold IPFS specific metadata
type IPFSBase struct {
	Base
	OverlayFileMeta

	/* The filesystem context should be set prior to `Attach`
	During `fs.Attach`, a new reference to `fs` should be created
	with its fs-context + fs-cancel populated with one derived from the existing fs context

	This context is expected to be valid for the lifetime of the file system
	and canceled on `Close` by references returned from `Attach` only.
	*/
	filesystemCtx    context.Context
	filesystemCancel context.CancelFunc

	/* During `file.Walk` a new reference to `file` should be created
	with its op-context + op-cancel populated with one derived from the existing fs context
	This context is expected to be valid as long as the file is being referenced by a particular FID
	it should be canceled during `Close`

	for the lifetime of the file system
	to be used during operations, such as `Open`, `Read` etc.
	and canceled on `Close` by references returned from `Attach` only.
	*/

	operationsCtx    context.Context
	operationsCancel context.CancelFunc

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

//func newIPFSBase(ctx context.Context, path corepath.Resolved, kind p9.FileMode, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) IPFSBase {
func newIPFSBase(ctx context.Context, coreNamespace string, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) IPFSBase {
	base := IPFSBase{
		Base:          newBase(ops...),
		coreNamespace: coreNamespace,
		core:          core,
		filesystemCtx: ctx,
	}

	options := nodeopts.AttachOps(ops...)
	if options.Parent != nil { // parent is optional
		parentRef, ok := options.Parent.(walkRef) // interface is not
		if !ok {
			panic("parent node lacks overlay traversal methods")
		}
		base.OverlayFileMeta.parent = parentRef
	}
	return base
}

func (ib *IPFSBase) clone() IPFSBase {
	newFid := IPFSBase{
		Base:            ib.Base.clone(),
		OverlayFileMeta: ib.OverlayFileMeta,
		coreNamespace:   ib.coreNamespace,
		core:            ib.core,
		filesystemCtx:   ib.filesystemCtx,
	}
	return newFid
}

func (ib *IPFSBase) Fork() (IPFSBase, error) {
	newFid := ib.clone()
	err := newFid.newOperations()

	return newFid, err
}

func (ib *IPFSBase) newFilesystem() error {
	if err := ib.checkFSCtx(); err != nil {
		return err
	}
	ib.filesystemCtx, ib.filesystemCancel = context.WithCancel(ib.filesystemCtx)
	return nil
}

func (ib *IPFSBase) newOperations() error {
	if err := ib.checkFSCtx(); err != nil {
		return err
	}
	ib.operationsCtx, ib.operationsCancel = context.WithCancel(ib.filesystemCtx)
	return nil
}

func (ib *IPFSBase) checkFSCtx() error {
	select {
	case <-ib.filesystemCtx.Done():
		return ib.filesystemCtx.Err()
	default:
		return nil
	}
}

func (ib *IPFSBase) String() string {
	return gopath.Join(append([]string{ib.coreNamespace}, ib.Base.String())...)
}

func (ib *IPFSBase) CorePath(names ...string) corepath.Path {
	return corepath.Join(rootPath(ib.coreNamespace), append(ib.Trail, names...)...)
}

func (b *IPFSBase) Flush() error {
	b.Logger.Debugf("flush requested: {%d}%q", b.Qid.Path, b.String())
	return nil
}

func (b *Base) Close() error {
	b.Logger.Debugf("closing: {%d}%q", b.Qid.Path, b.String())
	b.closed = true
	return nil
}

func (ib *IPFSBase) Close() error {
	lastErr := ib.Base.Close()
	if lastErr != nil {
		ib.Logger.Error(lastErr)
	}

	if ib.filesystemCancel != nil {
		/* TODO: only do this on the last close to the root
		if ib.proxy != nil {
		    if err := ib.proxy.Close(); err != nil {
				ib.Logger.Error(err)
			}
			lastErr = err
		}
		*/
		ib.filesystemCancel()
	}

	if ib.operationsCancel != nil {
		ib.operationsCancel()
	}

	return lastErr
}

func (b *Base) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	//b.Logger.Debugf("GetAttr {%d}:%q", b.Qid.Path, b.String())

	return *b.Qid, *b.metaMask, *b.meta, nil
}

func (b *IPFSBase) callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(b.filesystemCtx, 30*time.Second)
}

func (b *IPFSBase) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	b.Logger.Debugf("Open")
	return *b.Qid, 0, nil
}

func (ib *IPFSBase) step(self walkRef, name string) (walkRef, error) {
	if ib.Qid.Type != p9.TypeDir {
		return nil, ENOTDIR
	}

	if ib.closed == true {
		return nil, errors.New("TODO: ref was previously closed err")
	}

	ib.Trail = append(ib.Trail, name)
	ib.modified = true
	return self, nil
}

func (ib *IPFSBase) backtrack(self walkRef) (walkRef, error) {
	// if we're the root
	if len(ib.Trail) == 0 {
		// return our parent, or ourselves if we don't have one
		if ib.parent != nil {
			return ib.parent, nil
		}
		return self, nil
	}

	// otherwise step back
	ib.Trail = ib.Trail[:len(ib.Trail)-1]

	return self, nil
}

func (b *Base) CheckWalk() error {
	if b.closed {
		return errors.New("TODO: already closed msg")
	}
	return nil
}
