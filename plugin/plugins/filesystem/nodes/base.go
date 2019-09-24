package fsnodes

import (
	"context"
	"errors"
	"fmt"

	"github.com/djdv/p9/p9"
	"github.com/djdv/p9/unimplfs"
	files "github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

const ( //device - attempts to comply with standard multicodec table
	dMemory = 0x2f // generic "path"
	dIPFS   = 0xe4
)

var _ p9.File = (*Base)(nil)

// Base is a foundational file system node that provides common file metadata as well as stubs for unimplemented methods
type Base struct {
	// Provide stubs for unimplemented methods
	unimplfs.NoopFile
	p9.DefaultWalkGetAttr

	// Storage for file's metadata
	Qid      p9.QID
	meta     p9.Attr
	metaMask p9.AttrMask

	// The parent context should be set prior to calling attach (usually in a consturctor)
	// The Base filesystemCtx is derived from the parent context during Base.Attach
	// and should be valid for the lifetime of the file system
	// The fs context should be used to derive operation specific contexts from during calls
	// A cancel should cascade down, and invalidate all derived contexts
	// A call to fs.Close should leave no operations lingering
	parentCtx        context.Context
	filesystemCtx    context.Context
	filesystemCancel context.CancelFunc
	Logger           logging.EventLogger

	parent p9.File // parent must be set, it is used to handle ".." requests; nodes without a parent must point back to themselves
	child  p9.File // child is an optional field, used to hand off walk requests to another file system
	root   bool    // should be set to true on Attach if parent == self, this triggers the filesystemCancel on Close
	open   bool    // should be set to true on Open and checked during Walk; do not walk open references walk(5)
}

// IPFSBase is much like Base but extends it to hold IPFS specific metadata
type IPFSBase struct {
	Base

	Path corepath.Resolved
	core coreiface.CoreAPI

	// you will typically want to derive a context from the Base context within one operation (like Open)
	// use it with the CoreAPI for something (like Get)
	// and cancel it in another operation (like Close)
	// that pointer should be stored here between calls
	operationsCancel context.CancelFunc

	// operation handle storage
	file      files.File
	directory *directoryStream
}

// Base Attach should be called by all supersets during their Attach
// to initialize the file system context
func (b *Base) Attach() (p9.File, error) {
	if b.parentCtx == nil {
		return nil, errors.New("Parent context was not set, no way to derive")
	}

	select {
	case <-b.parentCtx.Done():
		return nil, fmt.Errorf("Parent is done: %s", b.parentCtx.Err())
	default:
		break
	}

	if b.filesystemCtx != nil {
		return nil, errors.New("Already attached")
	}
	b.filesystemCtx, b.filesystemCancel = context.WithCancel(b.parentCtx)

	return b, nil
}

// Base Close should be called in all superset Close methods in order to
// close child subsystems and cancel the file system context
func (b *Base) Close() error {
	b.Logger.Debugf("closing: {%v}", b.Qid.Path)

	if b.root {
		b.filesystemCancel()
	}

	var err error
	if b.child != nil {
		if err = b.child.Close(); err != nil {
			b.Logger.Error(err)
		}
	}

	return err
}

func (ib *IPFSBase) Close() error {
	ib.Logger.Debugf("closing:{%v}%q", ib.Qid, ib.Path.String())
	var lastErr error
	if ib.operationsCancel != nil {
		ib.operationsCancel()
	}

	if err := ib.Base.Close(); err != nil {
		ib.Logger.Error(err)
		lastErr = err
	}

	if ib.file != nil {
		if err := ib.file.Close(); err != nil {
			ib.Logger.Error(err)
			lastErr = err
		}
	}

	return lastErr
}

type directoryStream struct {
	entryChan <-chan coreiface.DirEntry
	cursor    uint64
	eos       bool // have seen end of stream?
	err       error
}

type walkRef interface {
	p9.File
	Parent() p9.File
	Child() p9.File
}

func (b *Base) Parent() p9.File {
	return b.parent
}

func (b *Base) Child() p9.File {
	return b.child
}
