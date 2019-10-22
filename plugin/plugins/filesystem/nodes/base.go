package fsnodes

import (
	"context"
	"errors"
	gopath "path"
	"time"

	"github.com/hugelgupf/p9/p9"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	fsutils "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/utils"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

const ( //device - attempts to comply with standard multicodec table
	dMemory = 0x2f // generic "path"
	dIPFS   = 0xe4
)

type p9Path = uint64

// Base provides a foundation to build file system nodes which contain file meta data
// as well as some base methods
type Base struct {
	Trail []string // FS "breadcrumb" trail from node's root

	// Storage for file's metadata
	qid      *p9.QID
	meta     *p9.Attr
	metaMask *p9.AttrMask
	Logger   logging.EventLogger

	closed   bool // set to true upon close; this reference should not be used again for anything
	modified bool // set to true when the `Trail` has been modified (usually by `Step`)
	// reset to false when `qid` has been populated with the current path in `Trail` (usually by `QID`)

}

func newBase(ops ...nodeopts.AttachOption) Base {
	options := nodeopts.AttachOps(ops...)

	return Base{
		Logger:   options.Logger,
		qid:      new(p9.QID),
		meta:     new(p9.Attr),
		metaMask: new(p9.AttrMask),
	}
}

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
	options := nodeopts.AttachOps(ops...)
	return IPFSBase{
		Base: newBase(ops...),
		OverlayFileMeta: OverlayFileMeta{
			parent: options.Parent,
		},
		coreNamespace: coreNamespace,
		core:          core,
		filesystemCtx: ctx,
	}
}

/* general helpers */

func (b *Base) ninePath() p9Path { return b.qid.Path }
func (b *Base) String() string   { return gopath.Join(b.Trail...) }

func (ib *IPFSBase) String() string {
	return gopath.Join(append([]string{ib.coreNamespace}, ib.Base.String())...)
}

// see filesystemCtx section in IPFSBase comments
func (ib *IPFSBase) forkFilesystem() error {
	if err := ib.filesystemCtx.Err(); err != nil {
		return err
	}
	ib.filesystemCtx, ib.filesystemCancel = context.WithCancel(ib.filesystemCtx)
	return nil
}

// see operationsCtx section in IPFSBase comments
func (ib *IPFSBase) forkOperations() error {
	if err := ib.filesystemCtx.Err(); err != nil {
		return err
	}
	ib.operationsCtx, ib.operationsCancel = context.WithCancel(ib.filesystemCtx)
	return nil
}

func (b *IPFSBase) callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(b.filesystemCtx, 30*time.Second)
}

func (ib *IPFSBase) CorePath(names ...string) corepath.Path {
	return corepath.Join(rootPath(ib.coreNamespace), append(ib.Trail, names...)...)
}

/* base operation methods to build on */

func (b *Base) close() error {
	b.Logger.Debugf("closing: {%d}%q", b.qid.Path, b.String())
	b.closed = true
	return nil
}

func (ib *IPFSBase) close() error {
	lastErr := ib.Base.close()
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
		ib.filesystemCancel()
		*/
	}

	if ib.operationsCancel != nil {
		ib.operationsCancel()
	}

	return lastErr
}

func (b *Base) getAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	//b.Logger.Debugf("GetAttr {%d}:%q", b.qid.Path, b.String())

	return *b.qid, *b.metaMask, *b.meta, nil
}

/* WalkRef relevant */

func (b *Base) checkWalk() error {
	if b.closed {
		return errors.New("TODO: already closed msg")
	}
	return nil
}

func (b *Base) qID() (p9.QID, error) { return *b.qid, nil }
func (b *Base) clone() Base          { return *b }

func (ib *IPFSBase) clone() IPFSBase {
	return IPFSBase{
		Base:            ib.Base.clone(),
		OverlayFileMeta: ib.OverlayFileMeta,
		coreNamespace:   ib.coreNamespace,
		core:            ib.core,
		filesystemCtx:   ib.filesystemCtx,
	}
}

// see fsutils.WalkRef.Fork documentation
func (b *Base) fork() Base {
	newFid := b.clone()

	tLen := len(b.Trail)
	newFid.Trail = b.Trail[:tLen:tLen] // make a new path slice for the new reference

	return newFid
}

func (ib *IPFSBase) fork() (IPFSBase, error) {
	newFid := ib.clone()
	err := newFid.forkOperations()
	if err != nil {
		return IPFSBase{}, err
	}

	return newFid, nil
}

// see fsutils.WalkRef.Step documentation
func (b *Base) step(self fsutils.WalkRef, name string) (fsutils.WalkRef, error) {
	if b.qid.Type != p9.TypeDir {
		return nil, ENOTDIR
	}

	if b.closed {
		return nil, errors.New("ref was previously closed") //TODO: use a 9P error value
	}

	tLen := len(b.Trail)
	b.Trail = append(b.Trail[:tLen:tLen], name)
	b.modified = true
	return self, nil
}

// see fsutils.WalkRef.Backtrack documentation
func (ib *IPFSBase) backtrack(self fsutils.WalkRef) (fsutils.WalkRef, error) {
	// if we're a root return our parent, or ourselves if we don't have one
	if len(ib.Trail) == 0 {
		if ib.parent != nil {
			return ib.parent, nil
		}
		return self, nil
	}

	// otherwise step back
	ib.Trail = ib.Trail[:len(ib.Trail)-1]

	return self, nil
}
